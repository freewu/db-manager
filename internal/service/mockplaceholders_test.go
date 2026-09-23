package service

import (
	"strings"
	"testing"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
)

func TestMockPlaceholdersRoundTripThroughTheService(t *testing.T) {
	manager, _ := managerOnDefaultDir(t)

	if list, err := manager.ListMockPlaceholders(); err != nil || len(list) != 0 {
		t.Fatalf("a fresh install listed %v (%v), want nothing", list, err)
	}

	// A user types the `@` they read in a template; the stored name is bare.
	saved, err := manager.SaveMockPlaceholder(models.MockPlaceholder{
		Name:        "@orderNo",
		Template:    "  SO@date(yyyy)@natural(1000, 9999)  ",
		Description: "  订单号  ",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.Name != "orderNo" || saved.Template != "SO@date(yyyy)@natural(1000, 9999)" || saved.Description != "订单号" {
		t.Fatalf("save returned %+v, want the trimmed fields", saved)
	}

	list, err := manager.ListMockPlaceholders()
	if err != nil || len(list) != 1 || list[0].Name != "orderNo" {
		t.Fatalf("list = %v (%v), want the saved placeholder", list, err)
	}

	if err := manager.DeleteMockPlaceholder("@orderNo"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, err := manager.ListMockPlaceholders(); err != nil || len(list) != 0 {
		t.Fatalf("after delete: %v (%v), want nothing", list, err)
	}
	// Deleting something that is not there is how a stale settings page behaves.
	if err := manager.DeleteMockPlaceholder("orderNo"); err != nil {
		t.Fatalf("delete again: %v", err)
	}
}

func TestMockPlaceholderNamesAreChecked(t *testing.T) {
	manager, _ := managerOnDefaultDir(t)

	cases := []struct {
		name string
		ok   bool
	}{
		{"orderNo", true},
		{"@orderNo", true},
		{"_private", true},
		{"order_no_2", true},
		{"订单号", false}, // the engine cannot write it, so it must not be stored
		{"order no", false},
		{"2fast", false},
		{"order-no", false},
		{"order.no", false},
		{"", false},
		{"@", false},
		{strings.Repeat("a", maxMockNameRunes+1), false},
	}
	for _, c := range cases {
		_, err := manager.SaveMockPlaceholder(models.MockPlaceholder{Name: c.name, Template: "@word"})
		if c.ok && err != nil {
			t.Errorf("save %q: %v, want it accepted", c.name, err)
		}
		if !c.ok && !apperr.Is(err, apperr.CodeInvalidConfig) {
			t.Errorf("save %q: %v, want an invalid-config refusal", c.name, err)
		}
	}
}

func TestMockPlaceholdersNeedATemplateAndStaySmall(t *testing.T) {
	manager, _ := managerOnDefaultDir(t)

	cases := []struct {
		name           string
		template, desc string
	}{
		{"no template", "   ", ""},
		{"template too long", strings.Repeat("a", maxMockTemplateRunes+1), ""},
		{"description too long", "@word", strings.Repeat("a", maxMockDescriptionRunes+1)},
	}
	for _, c := range cases {
		_, err := manager.SaveMockPlaceholder(models.MockPlaceholder{
			Name: "orderNo", Template: c.template, Description: c.desc,
		})
		if !apperr.Is(err, apperr.CodeInvalidConfig) {
			t.Errorf("%s: %v, want an invalid-config refusal", c.name, err)
		}
	}

	// A template that does not compile is *not* refused here: the engine that
	// reads it lives in the window, and this layer only owns the shape.
	if _, err := manager.SaveMockPlaceholder(models.MockPlaceholder{
		Name: "typo", Template: "@nope(1)",
	}); err != nil {
		t.Fatalf("a template the window will refuse is still stored: %v", err)
	}
}
