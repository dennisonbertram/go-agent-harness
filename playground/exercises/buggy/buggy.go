package main

// SumSlice returns the sum of the elements in the nums slice.
func SumSlice(nums []int) int {
	total := 0
	for i := 0; i < len(nums); i++ {
		total += nums[i]
	}
	return total
}
