package modelstore

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultPathUsesHarnessDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got := DefaultPath()
	if filepath.Base(got) != "models.json" || filepath.Base(filepath.Dir(got)) != ".harness" {
		t.Fatalf("DefaultPath() = %q, want .harness/models.json", got)
	}
}

func TestProviderNamesAreSorted(t *testing.T) {
	store := New()
	store.Providers["zeta"] = Provider{Name: "zeta"}
	store.Providers["alpha"] = Provider{Name: "alpha"}
	if got, want := store.ProviderNames(), []string{"alpha", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ProviderNames() = %v, want %v", got, want)
	}
}

func TestServiceSetCostPersistsUserPricing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	service, err := NewService(path)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	service.SeedModels("provider", []Model{{ID: "model"}})
	if err := service.SetCost("provider", "model", 2.5, 7.5); err != nil {
		t.Fatalf("SetCost: %v", err)
	}

	reloaded, err := NewService(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	_, fetched := reloaded.Snapshot()
	model := fetched["provider"].Models[0]
	if model.InputCost == nil || *model.InputCost != 2.5 || model.OutputCost == nil || *model.OutputCost != 7.5 {
		t.Fatalf("persisted pricing = input %v output %v", model.InputCost, model.OutputCost)
	}
	if model.CostSource != CostFromUser {
		t.Fatalf("cost source = %q, want %q", model.CostSource, CostFromUser)
	}
}
