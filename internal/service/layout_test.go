package service

import (
	"sort"
	"strings"
	"testing"

	"dbmanager/internal/config"
	"dbmanager/internal/models"
)

// The explorer arrangement is pure bookkeeping over the profile store: no
// driver, no session. These tests drive Manager against a temp store holding
// hand-written profiles, so the ids in the expectations are the ids in the
// fixtures.

func layoutManager(t *testing.T) (*Manager, *config.Store) {
	t.Helper()
	store, err := config.NewAt(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return &Manager{store: store}, store
}

// seedProfiles stores profiles named after their ids, in the order given, which
// is the file order the layout falls back to.
func seedProfiles(t *testing.T, store *config.Store, ids ...string) {
	t.Helper()
	for _, id := range ids {
		_, err := store.Upsert(models.ConnectionConfig{
			ID:     id,
			Name:   strings.ToUpper(id),
			Driver: models.DriverMySQL,
		})
		if err != nil {
			t.Fatalf("seed profile %s: %v", id, err)
		}
	}
}

func group(id, name string, order int) models.ConnectionGroup {
	return models.ConnectionGroup{ID: id, Name: name, Order: order}
}

func item(id string, order int) models.ConnectionPlacement {
	return models.ConnectionPlacement{ID: id, Order: order}
}

func member(id, groupID string, order int) models.ConnectionPlacement {
	return models.ConnectionPlacement{ID: id, GroupID: groupID, Order: order}
}

// draw renders an arrangement the way the explorer shows it: top-level entries
// in order, with the members of a group indented under its name. It is what the
// tests compare, because a broken *order* is the bug this file is about.
func draw(layout models.ConnectionLayout) string {
	type row struct {
		order int
		lines []string
	}
	rows := make([]row, 0, len(layout.Groups)+len(layout.Items))
	for _, group := range layout.Groups {
		lines := []string{group.Name}
		members := make([]models.ConnectionPlacement, 0)
		for _, entry := range layout.Items {
			if entry.GroupID == group.ID {
				members = append(members, entry)
			}
		}
		sort.SliceStable(members, func(i, j int) bool { return members[i].Order < members[j].Order })
		for _, entry := range members {
			lines = append(lines, "  "+entry.ID)
		}
		rows = append(rows, row{order: group.Order, lines: lines})
	}
	for _, entry := range layout.Items {
		if entry.GroupID != "" {
			continue
		}
		rows = append(rows, row{order: entry.Order, lines: []string{entry.ID}})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].order < rows[j].order })
	out := make([]string, 0, len(rows))
	for _, current := range rows {
		out = append(out, current.lines...)
	}
	return strings.Join(out, "\n")
}

func TestConnectionLayoutKeepsTheArrangementThatWasStored(t *testing.T) {
	manager, store := layoutManager(t)
	seedProfiles(t, store, "p1", "p2", "p3", "p4")
	if err := store.SaveLayout(models.ConnectionLayout{
		Groups: []models.ConnectionGroup{group("g1", "prod", 0)},
		Items: []models.ConnectionPlacement{
			member("p1", "g1", 0),
			member("p2", "g1", 1),
			item("p3", 1),
			item("p4", 2),
		},
	}); err != nil {
		t.Fatalf("seed layout: %v", err)
	}

	layout, err := manager.ConnectionLayout()
	if err != nil {
		t.Fatalf("read layout: %v", err)
	}
	if got, want := draw(layout), "prod\n  p1\n  p2\np3\np4"; got != want {
		t.Fatalf("layout =\n%s\nwant\n%s", got, want)
	}
}

// A profile saved after the arrangement was stored — and every profile written
// by a build that knew nothing about groups — has to show up instead of falling
// through the cracks.
func TestConnectionLayoutAppendsProfilesTheLayoutDoesNotMention(t *testing.T) {
	manager, store := layoutManager(t)
	seedProfiles(t, store, "p1", "p2", "p3")
	if err := store.SaveLayout(models.ConnectionLayout{
		Groups: []models.ConnectionGroup{group("g1", "prod", 0)},
		Items:  []models.ConnectionPlacement{member("p1", "g1", 0), item("p3", 1)},
	}); err != nil {
		t.Fatalf("seed layout: %v", err)
	}

	layout, err := manager.ConnectionLayout()
	if err != nil {
		t.Fatalf("read layout: %v", err)
	}
	// p2 is missing from the stored layout and lands at the bottom; p1 stays in
	// its group and p3 above it, so nothing the user arranged moved.
	if got, want := draw(layout), "prod\n  p1\np3\np2"; got != want {
		t.Fatalf("layout =\n%s\nwant\n%s", got, want)
	}
	// Two unplaced profiles keep the file order they were written in.
	seedProfiles(t, store, "p4")
	layout, err = manager.ConnectionLayout()
	if err != nil {
		t.Fatalf("read layout: %v", err)
	}
	if got, want := draw(layout), "prod\n  p1\np3\np2\np4"; got != want {
		t.Fatalf("layout =\n%s\nwant\n%s", got, want)
	}
}

func TestConnectionLayoutDropsEntriesThatNoLongerExist(t *testing.T) {
	manager, store := layoutManager(t)
	seedProfiles(t, store, "p1")
	if err := store.SaveLayout(models.ConnectionLayout{
		Groups: []models.ConnectionGroup{group("g1", "prod", 0), group("g1", "duplicate", 1), group("", "nameless", 2)},
		Items: []models.ConnectionPlacement{
			item("gone", 0),
			member("p1", "g1", 0),
			member("p1", "g1", 9), // a second entry for one profile
			item("p1", 0),         // … and a third
			item("p1", 1),
			member("p2", "nope", 2), // no such profile: the group reference goes too
		},
	}); err != nil {
		t.Fatalf("seed layout: %v", err)
	}

	layout, err := manager.ConnectionLayout()
	if err != nil {
		t.Fatalf("read layout: %v", err)
	}
	if got, want := draw(layout), "prod\n  p1"; got != want {
		t.Fatalf("layout =\n%s\nwant\n%s", got, want)
	}
	if len(layout.Groups) != 1 {
		t.Fatalf("groups = %+v, want only the first g1", layout.Groups)
	}
	if len(layout.Items) != 1 {
		t.Fatalf("items = %+v, want only the first entry for p1", layout.Items)
	}
}

// An entry pointing at a group that is gone is a connection the user still
// wants; it goes back to the top level rather than disappearing with the group.
func TestConnectionLayoutMovesEntriesOfUnknownGroupsToTheTopLevel(t *testing.T) {
	manager, store := layoutManager(t)
	seedProfiles(t, store, "p1")
	if err := store.SaveLayout(models.ConnectionLayout{
		Items: []models.ConnectionPlacement{member("p1", "deleted", 0)},
	}); err != nil {
		t.Fatalf("seed layout: %v", err)
	}

	layout, err := manager.ConnectionLayout()
	if err != nil {
		t.Fatalf("read layout: %v", err)
	}
	if got, want := draw(layout), "p1"; got != want {
		t.Fatalf("layout =\n%s\nwant\n%s", got, want)
	}
}

func TestSaveConnectionLayoutTidiesAndPersists(t *testing.T) {
	manager, store := layoutManager(t)
	seedProfiles(t, store, "p1", "p2", "p3")

	// What a drag sends: the members of a group numbered on their own, orders
	// that tie, and a placeholder for a profile that is not there.
	saved, err := manager.SaveConnectionLayout(models.ConnectionLayout{
		Groups: []models.ConnectionGroup{group("g1", "prod", 0), group("g2", "dev", 0)},
		Items: []models.ConnectionPlacement{
			member("p2", "g1", 0),
			member("p1", "g1", 0),
			item("p3", 0),
			item("ghost", 0),
		},
	})
	if err != nil {
		t.Fatalf("save layout: %v", err)
	}
	// g1 and g2 tie, so the stored order decides; the members of g1 tie as well,
	// so p2 — stored first — stays first. Every order is rewritten to a
	// 0..n-1 sequence, and the unknown id is gone.
	if got, want := draw(saved), "prod\n  p2\n  p1\ndev\np3"; got != want {
		t.Fatalf("saved layout =\n%s\nwant\n%s", got, want)
	}
	for i, group := range saved.Groups {
		if group.Order != i {
			t.Fatalf("group %s has order %d, want %d", group.ID, group.Order, i)
		}
	}

	// What was saved is what comes back, and saving it again changes nothing:
	// the write path and the read path agree.
	read, err := manager.ConnectionLayout()
	if err != nil {
		t.Fatalf("read layout: %v", err)
	}
	if draw(read) != draw(saved) {
		t.Fatalf("read back =\n%s\nwant\n%s", draw(read), draw(saved))
	}
	again, err := manager.SaveConnectionLayout(read)
	if err != nil {
		t.Fatalf("save layout again: %v", err)
	}
	if draw(again) != draw(saved) {
		t.Fatalf("second save =\n%s\nwant\n%s", draw(again), draw(saved))
	}
}

func TestSaveConnectionLayoutKeepsProfilesTheCallerForgot(t *testing.T) {
	manager, store := layoutManager(t)
	seedProfiles(t, store, "p1", "p2")

	saved, err := manager.SaveConnectionLayout(models.ConnectionLayout{
		Groups: []models.ConnectionGroup{group("g1", "prod", 0)},
		Items:  []models.ConnectionPlacement{member("p1", "g1", 0)},
	})
	if err != nil {
		t.Fatalf("save layout: %v", err)
	}
	if got, want := draw(saved), "prod\n  p1\np2"; got != want {
		t.Fatalf("saved layout =\n%s\nwant\n%s", got, want)
	}
}

func TestGroupsAndConnectionsShareTheTopLevelOrder(t *testing.T) {
	manager, store := layoutManager(t)
	seedProfiles(t, store, "p1", "p2", "p3")

	// A connection above a group, and another below it — the drag the user makes
	// when they want a server to sit next to the folder it does not belong to.
	saved, err := manager.SaveConnectionLayout(models.ConnectionLayout{
		Groups: []models.ConnectionGroup{group("g1", "prod", 1)},
		Items: []models.ConnectionPlacement{
			item("p1", 0),
			member("p2", "g1", 0),
			item("p3", 2),
		},
	})
	if err != nil {
		t.Fatalf("save layout: %v", err)
	}
	if got, want := draw(saved), "p1\nprod\n  p2\np3"; got != want {
		t.Fatalf("saved layout =\n%s\nwant\n%s", got, want)
	}
}

func TestSaveConnectionGroupCreatesAndRenames(t *testing.T) {
	manager, store := layoutManager(t)
	seedProfiles(t, store, "p1")

	if _, err := manager.SaveConnectionGroup(models.ConnectionGroup{Name: "   "}); err == nil {
		t.Fatal("expected an error for a blank name")
	}
	if _, err := manager.SaveConnectionGroup(models.ConnectionGroup{Name: strings.Repeat("x", maxGroupName+1)}); err == nil {
		t.Fatal("expected an error for an over-long name")
	}

	created, err := manager.SaveConnectionGroup(models.ConnectionGroup{Name: "  prod  "})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if created.ID == "" || created.Name != "prod" {
		t.Fatalf("created = %+v, want a fresh id and a trimmed name", created)
	}
	// A second group goes below the first, and both go below the loose profile
	// that was already there: a new folder is appended, it does not push aside
	// what the user arranged earlier.
	second, err := manager.SaveConnectionGroup(models.ConnectionGroup{Name: "dev"})
	if err != nil {
		t.Fatalf("create second group: %v", err)
	}
	layout, err := manager.ConnectionLayout()
	if err != nil {
		t.Fatalf("read layout: %v", err)
	}
	if got, want := draw(layout), "p1\nprod\ndev"; got != want {
		t.Fatalf("layout =\n%s\nwant\n%s", got, want)
	}

	// Renaming keeps the position: p1 is still above the group.
	renamed, err := manager.SaveConnectionGroup(models.ConnectionGroup{ID: created.ID, Name: "production"})
	if err != nil {
		t.Fatalf("rename group: %v", err)
	}
	if renamed.ID != created.ID || renamed.Order != created.Order {
		t.Fatalf("renamed = %+v, want the same id and order as %+v", renamed, created)
	}
	layout, err = manager.ConnectionLayout()
	if err != nil {
		t.Fatalf("read layout: %v", err)
	}
	if got, want := draw(layout), "p1\nproduction\ndev"; got != want {
		t.Fatalf("layout =\n%s\nwant\n%s", got, want)
	}

	// An id the store does not know is a create, with an id of our own: the
	// frontend must never get back an id it made up.
	other, err := manager.SaveConnectionGroup(models.ConnectionGroup{ID: second.ID + "-stale", Name: "gone"})
	if err != nil {
		t.Fatalf("create from a stale id: %v", err)
	}
	if other.ID == second.ID+"-stale" {
		t.Fatalf("stale id %q was kept", other.ID)
	}
}

func TestDeleteConnectionGroupKeepsItsConnections(t *testing.T) {
	manager, store := layoutManager(t)
	seedProfiles(t, store, "p1", "p2", "p3")
	if _, err := manager.SaveConnectionLayout(models.ConnectionLayout{
		Groups: []models.ConnectionGroup{group("g1", "prod", 0), group("g2", "dev", 1)},
		Items: []models.ConnectionPlacement{
			member("p1", "g1", 0),
			member("p2", "g1", 1),
			member("p3", "g2", 0),
		},
	}); err != nil {
		t.Fatalf("seed layout: %v", err)
	}

	if err := manager.DeleteConnectionGroup("nope"); err != nil {
		t.Fatalf("deleting an unknown group must be a no-op, got %v", err)
	}
	if err := manager.DeleteConnectionGroup("  "); err == nil {
		t.Fatal("expected an error for a blank id")
	}
	if err := manager.DeleteConnectionGroup("g1"); err != nil {
		t.Fatalf("delete group: %v", err)
	}

	layout, err := manager.ConnectionLayout()
	if err != nil {
		t.Fatalf("read layout: %v", err)
	}
	// p1 and p2 are handed back below everything that was already there, in the
	// order they had inside the group.
	if got, want := draw(layout), "dev\n  p3\np1\np2"; got != want {
		t.Fatalf("layout =\n%s\nwant\n%s", got, want)
	}
	// The profiles themselves are untouched.
	profiles, err := store.Load()
	if err != nil {
		t.Fatalf("read profiles: %v", err)
	}
	if len(profiles) != 3 {
		t.Fatalf("profiles = %d, want 3", len(profiles))
	}
}

// Deleting a profile must take its entry with it, whether or not the layout was
// rewritten afterwards.
func TestDeletedProfileLeavesTheLayout(t *testing.T) {
	manager, store := layoutManager(t)
	seedProfiles(t, store, "p1", "p2")
	if _, err := manager.SaveConnectionLayout(models.ConnectionLayout{
		Groups: []models.ConnectionGroup{group("g1", "prod", 0)},
		Items: []models.ConnectionPlacement{
			member("p1", "g1", 0),
			item("p2", 1),
		},
	}); err != nil {
		t.Fatalf("seed layout: %v", err)
	}
	if err := manager.DeleteConnection("p1"); err != nil {
		t.Fatalf("delete profile: %v", err)
	}

	layout, err := manager.ConnectionLayout()
	if err != nil {
		t.Fatalf("read layout: %v", err)
	}
	if got, want := draw(layout), "prod\np2"; got != want {
		t.Fatalf("layout =\n%s\nwant\n%s", got, want)
	}
	// The group stays even when it is empty: it is where the user put it, and an
	// empty folder is still a place to drag the next connection into.
	if len(layout.Groups) != 1 || layout.Groups[0].ID != "g1" {
		t.Fatalf("groups = %+v, want the empty g1 to remain", layout.Groups)
	}
}
