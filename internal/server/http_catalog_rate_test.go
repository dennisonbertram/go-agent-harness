package server

import "testing"

func TestCatalogRateReturnsOnlyKnownCuratedPricing(t *testing.T) {
	server := &Server{catalog: testCatalog()}
	input, output, ok := server.catalogRate("openai", "gpt-4.1")
	if !ok || input == nil || output == nil || *input != 2 || *output != 8 {
		t.Fatalf("catalogRate(openai, gpt-4.1) = %v, %v, %v", input, output, ok)
	}
	if _, _, ok := server.catalogRate("missing", "gpt-4.1"); ok {
		t.Fatal("unknown provider unexpectedly had catalog pricing")
	}
	if _, _, ok := server.catalogRate("openai", "missing"); ok {
		t.Fatal("unknown model unexpectedly had catalog pricing")
	}
	if _, _, ok := (*Server)(nil).catalogRate("openai", "gpt-4.1"); ok {
		t.Fatal("nil server unexpectedly had catalog pricing")
	}
}
