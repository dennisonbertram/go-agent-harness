package providercatalog

import (
	"testing"
	"time"
)

func TestShippedFilesSmoke(t *testing.T) {
	cat, err := Load("../../catalog/providers")
	if err != nil {
		t.Fatalf("load shipped provider files: %v", err)
	}
	t.Logf("loaded %d providers: %v", len(cat.Providers), cat.IDs())
	pc := cat.ToPricingCatalog()
	mc := cat.ToModelCatalog()
	priced := 0
	for _, p := range pc.Providers {
		priced += len(p.Models)
	}
	t.Logf("pricing catalog: %d providers, %d priced models", len(pc.Providers), priced)
	t.Logf("model catalog:   %d usable providers", len(mc.Providers))
	t.Logf("non-USD:         %v", cat.NonUSDProviders())
	t.Logf("stale >30d:      %v", cat.StaleProviders(time.Now(), 30*24*time.Hour))
}
