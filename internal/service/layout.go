package service

import (
	"sort"
	"strings"

	"github.com/google/uuid"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
)

// This file owns the arrangement of the connection explorer: which groups
// exist, which profile sits in which group, and in what order.
//
// The rule that holds it together is that the arrangement is *derived* from the
// profiles rather than standing next to them: a profile the stored arrangement
// does not mention is appended instead of disappearing, an entry for a profile
// that no longer exists is dropped, and a reference to a group that is gone
// falls back to the top level. Reading and writing both go through
// placeProfiles, so the tree the user sees and the file on disk cannot disagree
// — and a layout written by a later build (or hand-edited) degrades instead of
// breaking.

// maxGroupName bounds a group name, counted in runes like every other
// user-visible name in the store.
const maxGroupName = 60

// ConnectionLayout returns the explorer arrangement as it should be drawn.
//
// Every saved profile has a place in the result, whether or not the stored
// layout knows about it: a profile saved a moment ago — and every profile
// written by a build that had no groups — shows up at the bottom of the top
// level.
func (m *Manager) ConnectionLayout() (models.ConnectionLayout, error) {
	profiles, err := m.storeRef().Load()
	if err != nil {
		return models.ConnectionLayout{}, apperr.Wrap(apperr.CodeInternal, err, "read connection profiles")
	}
	stored, err := m.storeRef().LoadLayout()
	if err != nil {
		return models.ConnectionLayout{}, apperr.Wrap(apperr.CodeInternal, err, "read the connection layout")
	}
	return placeProfiles(profiles, stored), nil
}

// SaveConnectionLayout stores a new arrangement and returns the tidied version.
//
// The frontend sends what it drew; this re-derives it against the profiles that
// exist, so a drag can never lose a connection and an entry for something that
// is gone is dropped rather than kept as a ghost. The returned layout is what
// the file now holds, which is also what the caller should draw next.
//
// Names are not validated here: this call says where things are, not what they
// are called. See SaveConnectionGroup.
func (m *Manager) SaveConnectionLayout(layout models.ConnectionLayout) (models.ConnectionLayout, error) {
	profiles, err := m.storeRef().Load()
	if err != nil {
		return models.ConnectionLayout{}, apperr.Wrap(apperr.CodeInternal, err, "read connection profiles")
	}
	out := placeProfiles(profiles, layout)
	if err := m.storeRef().SaveLayout(out); err != nil {
		return models.ConnectionLayout{}, apperr.Wrap(apperr.CodeInternal, err, "save the connection layout")
	}
	return out, nil
}

// SaveConnectionGroup creates or renames a group and returns the stored value.
//
// A new group lands at the bottom of the explorer, below everything already
// there. Renaming an existing one keeps the position it had: the user put it
// where they want it, and typing a new name is not a request to move it.
func (m *Manager) SaveConnectionGroup(group models.ConnectionGroup) (models.ConnectionGroup, error) {
	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" {
		return group, apperr.New(apperr.CodeInvalidConfig, "give the group a name")
	}
	if len([]rune(group.Name)) > maxGroupName {
		return group, apperr.New(
			apperr.CodeInvalidConfig,
			"the group name is too long (%d characters max)", maxGroupName,
		)
	}

	profiles, err := m.storeRef().Load()
	if err != nil {
		return group, apperr.Wrap(apperr.CodeInternal, err, "read connection profiles")
	}
	stored, err := m.storeRef().LoadLayout()
	if err != nil {
		return group, apperr.Wrap(apperr.CodeInternal, err, "read the connection layout")
	}
	layout := placeProfiles(profiles, stored)

	renamed := false
	for i := range layout.Groups {
		if group.ID != "" && layout.Groups[i].ID == group.ID {
			group.Order = layout.Groups[i].Order
			layout.Groups[i] = group
			renamed = true
			break
		}
	}
	if !renamed {
		// An id the store does not know is not an id we may hand back: a deleted
		// group leaves the frontend holding an id that must not come back to
		// life under a new name.
		group.ID = uuid.NewString()
		group.Order = nextTopOrder(layout.Groups, layout.Items)
		layout.Groups = append(layout.Groups, group)
	}

	out := placeProfiles(profiles, layout)
	if err := m.storeRef().SaveLayout(out); err != nil {
		return group, apperr.Wrap(apperr.CodeInternal, err, "save the connection layout")
	}
	// Answer with the group as it now stands: the caller draws what it gets, and
	// the order it holds is the one in the file rather than the one guessed here.
	for _, saved := range out.Groups {
		if saved.ID == group.ID {
			return saved, nil
		}
	}
	return group, nil
}

// DeleteConnectionGroup removes a group. The connections inside it are handed
// back to the top level, after everything already there and in the order they
// had: deleting a folder must never delete what is in it.
//
// An unknown id is a no-op, so a double click cannot fail.
func (m *Manager) DeleteConnectionGroup(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return apperr.New(apperr.CodeInvalidConfig, "group id is required")
	}

	profiles, err := m.storeRef().Load()
	if err != nil {
		return apperr.Wrap(apperr.CodeInternal, err, "read connection profiles")
	}
	stored, err := m.storeRef().LoadLayout()
	if err != nil {
		return apperr.Wrap(apperr.CodeInternal, err, "read the connection layout")
	}
	layout := placeProfiles(profiles, stored)

	found := false
	groups := make([]models.ConnectionGroup, 0, len(layout.Groups))
	for _, group := range layout.Groups {
		if group.ID == id {
			found = true
			continue
		}
		groups = append(groups, group)
	}
	if !found {
		return nil
	}

	order := nextTopOrder(layout.Groups, layout.Items)
	items := make([]models.ConnectionPlacement, 0, len(layout.Items))
	for _, item := range layout.Items {
		if item.GroupID == id {
			item.GroupID = ""
			item.Order = order
			order++
		}
		items = append(items, item)
	}

	tidied := placeProfiles(profiles, models.ConnectionLayout{Groups: groups, Items: items})
	if err := m.storeRef().SaveLayout(tidied); err != nil {
		return apperr.Wrap(apperr.CodeInternal, err, "save the connection layout")
	}
	return nil
}

// nextTopOrder returns an order that sorts after every top-level entry, so a
// caller appending a group — or re-homing a connection — can just keep
// counting. Top-level order is shared by the groups and the ungrouped
// connections, so both have to be looked at.
func nextTopOrder(groups []models.ConnectionGroup, items []models.ConnectionPlacement) int {
	next := 0
	for _, group := range groups {
		if group.Order >= next {
			next = group.Order + 1
		}
	}
	for _, item := range items {
		if item.GroupID == "" && item.Order >= next {
			next = item.Order + 1
		}
	}
	return next
}

// placeProfiles resolves a stored arrangement against the profiles that exist
// and renumbers it: one top-level sequence shared by the groups and the
// ungrouped connections, and a sequence per group for its members.
//
// It is the only place that interprets a layout, on the way in and on the way
// out, so the rules are stated exactly once:
//
//   - a profile without an entry is appended to the top level, in profile order
//   - an entry without a profile is dropped (the profile was deleted)
//   - two entries for one profile keep the first, the rest are dropped
//   - an entry pointing at a group that is gone becomes a top-level entry
//   - entries are ordered by Order, ties keeping the stored order, with a group
//     winning the tie against a connection
func placeProfiles(profiles []models.ConnectionConfig, want models.ConnectionLayout) models.ConnectionLayout {
	// Where each profile sits in the file: the fallback order for the profiles
	// the arrangement says nothing about.
	fileOrder := make(map[string]int, len(profiles))
	for i, profile := range profiles {
		if profile.ID == "" {
			continue
		}
		if _, seen := fileOrder[profile.ID]; !seen {
			fileOrder[profile.ID] = i
		}
	}

	groups := make([]models.ConnectionGroup, 0, len(want.Groups))
	inGroup := make(map[string][]models.ConnectionPlacement, len(want.Groups))
	for _, group := range want.Groups {
		if group.ID == "" {
			continue
		}
		if _, seen := inGroup[group.ID]; seen {
			continue
		}
		inGroup[group.ID] = []models.ConnectionPlacement{}
		groups = append(groups, group)
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Order < groups[j].Order })

	top := make([]models.ConnectionPlacement, 0, len(profiles))
	placed := make(map[string]bool, len(profiles))
	for _, item := range want.Items {
		if _, ok := fileOrder[item.ID]; !ok || placed[item.ID] {
			continue
		}
		placed[item.ID] = true
		if members, ok := inGroup[item.GroupID]; item.GroupID != "" && ok {
			inGroup[item.GroupID] = append(members, item)
			continue
		}
		item.GroupID = ""
		top = append(top, item)
	}

	// Profiles the arrangement never mentions go to the bottom of the top level,
	// in the order the file lists them (iterating `profiles` gives file order).
	order := nextTopOrder(groups, top)
	for _, profile := range profiles {
		if profile.ID == "" || placed[profile.ID] {
			continue
		}
		placed[profile.ID] = true
		top = append(top, models.ConnectionPlacement{ID: profile.ID, Order: order})
		order++
	}

	// Draw the top level: groups and ungrouped connections in one sequence.
	type slot struct {
		order int
		group int // index into groups, -1 for a connection
		item  int // index into top, -1 for a group
	}
	slots := make([]slot, 0, len(groups)+len(top))
	for i, group := range groups {
		slots = append(slots, slot{order: group.Order, group: i, item: -1})
	}
	for i, item := range top {
		slots = append(slots, slot{order: item.Order, group: -1, item: i})
	}
	sort.SliceStable(slots, func(i, j int) bool {
		if slots[i].order != slots[j].order {
			return slots[i].order < slots[j].order
		}
		// A group wins a tie, so what is inside it is drawn next to its name
		// rather than after whatever else shares the number.
		return slots[i].group >= 0 && slots[j].group < 0
	})

	out := models.ConnectionLayout{
		Groups: make([]models.ConnectionGroup, 0, len(groups)),
		Items:  make([]models.ConnectionPlacement, 0, len(placed)),
	}
	for i, current := range slots {
		if current.group >= 0 {
			group := groups[current.group]
			group.Order = i
			out.Groups = append(out.Groups, group)
			continue
		}
		item := top[current.item]
		item.Order = i
		out.Items = append(out.Items, item)
	}
	// Then each group's members, numbered from zero inside their own group.
	for _, group := range out.Groups {
		members := inGroup[group.ID]
		sort.SliceStable(members, func(i, j int) bool { return members[i].Order < members[j].Order })
		for i, item := range members {
			item.GroupID = group.ID
			item.Order = i
			out.Items = append(out.Items, item)
		}
	}
	return out
}
