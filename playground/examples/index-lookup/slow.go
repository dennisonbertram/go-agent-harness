package main

func ContainsStr(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func BuildIndex(items []string) map[string]bool {
	idx := map[string]bool{}
	for _, s := range items {
		idx[s] = true
	}
	return idx
}

func ContainsFast(idx map[string]bool, needle string) bool {
	return idx[needle]
}
