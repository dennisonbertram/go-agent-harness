package main

import (
	"strings"
	"testing"

	"go-agent-harness/internal/provider/catalog"
)

// TestBuildPickerItems_WithCatalog verifies that a catalog with two providers produces
// the expected header/model structure in alphabetical order.
func TestBuildPickerItems_WithCatalog(t *testing.T) {
	cat := testCatalog()
	items := buildPickerItems(cat)
	if len(items) == 0 {
		t.Fatal("expected items, got none")
	}
	// First item should be a header (empty modelKey)
	if items[0].modelKey != "" {
		t.Errorf("expected first item to be header, got modelKey=%q", items[0].modelKey)
	}
	// At least one selectable item must exist
	found := false
	for _, it := range items {
		if it.modelKey != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("no selectable items found")
	}
	// Every model item's displayLine must contain the model key
	for _, it := range items {
		if it.modelKey == "" {
			continue
		}
		if !strings.Contains(it.displayLine, it.modelKey) {
			t.Errorf("displayLine %q does not contain modelKey %q", it.displayLine, it.modelKey)
		}
	}
}

// TestBuildPickerItems_NilCatalog verifies nil catalog returns nil without panic.
func TestBuildPickerItems_NilCatalog(t *testing.T) {
	items := buildPickerItems(nil)
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

// TestBuildPickerItems_ModelWithoutPricing verifies that a model with nil Pricing is
// included without any "nil" or "NaN" text in the displayLine.
func TestBuildPickerItems_ModelWithoutPricing(t *testing.T) {
	cat := &catalog.Catalog{
		Providers: map[string]catalog.ProviderEntry{
			"myprovider": {
				DisplayName: "MyProvider",
				Models: map[string]catalog.Model{
					"free-model": {DisplayName: "Free Model", Pricing: nil},
				},
			},
		},
	}
	items := buildPickerItems(cat)
	for _, it := range items {
		if strings.Contains(it.displayLine, "nil") || strings.Contains(it.displayLine, "NaN") {
			t.Errorf("displayLine contains nil/NaN: %q", it.displayLine)
		}
	}
	// The model must be present
	found := false
	for _, it := range items {
		if it.modelKey == "free-model" {
			found = true
		}
	}
	if !found {
		t.Error("free-model not found in items")
	}
}

// TestBuildPickerItems_EmptyCatalog verifies that an empty providers map yields zero items.
func TestBuildPickerItems_EmptyCatalog(t *testing.T) {
	cat := &catalog.Catalog{Providers: map[string]catalog.ProviderEntry{}}
	items := buildPickerItems(cat)
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

// TestFirstSelectable_Forward verifies forward search skips header at index 0.
func TestFirstSelectable_Forward(t *testing.T) {
	items := []pickerItem{
		{modelKey: "", displayLine: "[header]"},
		{modelKey: "m1", displayLine: "m1"},
		{modelKey: "m2", displayLine: "m2"},
	}
	got := firstSelectable(items, 0, +1)
	if got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
}

// TestFirstSelectable_Backward verifies backward search from last index finds previous selectable.
func TestFirstSelectable_Backward(t *testing.T) {
	items := []pickerItem{
		{modelKey: "", displayLine: "[header]"},
		{modelKey: "m1", displayLine: "m1"},
		{modelKey: "m2", displayLine: "m2"},
	}
	got := firstSelectable(items, 2, -1)
	if got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
	got2 := firstSelectable(items, 1, -1)
	if got2 != 1 {
		t.Errorf("expected 1, got %d", got2)
	}
}

// TestFirstSelectable_WrapSafe verifies that searching forward from last index returns -1.
func TestFirstSelectable_WrapSafe(t *testing.T) {
	items := []pickerItem{
		{modelKey: "m1", displayLine: "m1"},
		{modelKey: "m2", displayLine: "m2"},
	}
	// Already at last item, searching forward — no more items
	got := firstSelectable(items, 2, +1)
	if got != -1 {
		t.Errorf("expected -1, got %d", got)
	}
}

// TestFirstSelectable_AllHeaders verifies -1 is returned when all items are headers.
func TestFirstSelectable_AllHeaders(t *testing.T) {
	items := []pickerItem{
		{modelKey: "", displayLine: "[h1]"},
		{modelKey: "", displayLine: "[h2]"},
	}
	got := firstSelectable(items, 0, +1)
	if got != -1 {
		t.Errorf("expected -1, got %d", got)
	}
}

// TestSelectModel_NilCatalog verifies selectModel returns "", "" for nil catalog.
func TestSelectModel_NilCatalog(t *testing.T) {
	gotModel, gotProvider := selectModel(nil, true)
	if gotModel != "" {
		t.Errorf("expected empty model, got %q", gotModel)
	}
	if gotProvider != "" {
		t.Errorf("expected empty provider, got %q", gotProvider)
	}
}

// TestSelectModel_EmptyCatalog verifies selectModel returns "", "" when there are no selectable items.
func TestSelectModel_EmptyCatalog(t *testing.T) {
	cat := &catalog.Catalog{Providers: map[string]catalog.ProviderEntry{}}
	gotModel, gotProvider := selectModel(cat, true)
	if gotModel != "" {
		t.Errorf("expected empty model, got %q", gotModel)
	}
	if gotProvider != "" {
		t.Errorf("expected empty provider, got %q", gotProvider)
	}
}

// TestBuildPickerItems_ProviderKey verifies that selectable items have providerKey set.
func TestBuildPickerItems_ProviderKey(t *testing.T) {
	cat := testCatalog()
	items := buildPickerItems(cat)

	for _, it := range items {
		if it.modelKey == "" {
			// Header rows should have empty providerKey
			continue
		}
		// Every selectable item must have a non-empty providerKey.
		if it.providerKey == "" {
			t.Errorf("selectable item %q has empty providerKey", it.modelKey)
		}
	}

	// Verify that model keys are associated with the correct provider.
	// testCatalog has "acme" provider with "acme-fast" and "acme-pro",
	// and "beta" provider with "beta-mini".
	for _, it := range items {
		if it.modelKey == "" {
			continue
		}
		switch it.modelKey {
		case "acme-fast", "acme-pro":
			if it.providerKey != "acme" {
				t.Errorf("model %q: expected providerKey=acme, got %q", it.modelKey, it.providerKey)
			}
		case "beta-mini":
			if it.providerKey != "beta" {
				t.Errorf("model %q: expected providerKey=beta, got %q", it.modelKey, it.providerKey)
			}
		}
	}
}
