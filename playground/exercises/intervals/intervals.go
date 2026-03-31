package main

import (
	"sort"
)

// MergeIntervals merges overlapping intervals.
func MergeIntervals(intervals [][]int) [][]int {
	if len(intervals) == 0 {
		return [][]int{}
	}
	// Sort intervals based on the start times.
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i][0] < intervals[j][0]
	})
	merged := make([][]int, 0, len(intervals))
	current := intervals[0]
	for i := 1; i < len(intervals); i++ {
		if intervals[i][0] <= current[1] { // Overlaps or adjacent
			if intervals[i][1] > current[1] {
				current[1] = intervals[i][1]
			}
		} else {
			merged = append(merged, current)
			current = intervals[i]
		}
	}
	merged = append(merged, current)
	return merged
}
