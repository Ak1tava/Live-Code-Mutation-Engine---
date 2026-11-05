package textkit

import "strings"

// Max returns the largest element of nums. nums must not be empty.
func Max(nums []int) int {
	m := nums[0]
	for i := 1; i < len(nums); i++ {
		if nums[i] > m {
			m = nums[i]
		}
	}
	return m
}

// IsPrime reports whether n is a prime number.
func IsPrime(n int) bool {
	if n < 2 {
		return false
	}
	for i := 2; i*i <= n; i++ {
		if n%i == 0 {
			return false
		}
	}
	return true
}

// Capitalize upper-cases the first letter of every word and lower-cases the rest.
func Capitalize(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}

// AtoiBase converts s to an integer using the digits of base.
// It returns 0 for an invalid base or an unknown digit.
func AtoiBase(s, base string) int {
	if len(base) < 2 {
		return 0
	}
	res := 0
	for _, r := range s {
		idx := strings.IndexRune(base, r)
		if idx == -1 {
			return 0
		}
		res = res*len(base) + idx
	}
	return res
}

// WindowSums returns the sums of every window of k consecutive elements.
func WindowSums(nums []int, k int) []int {
	if k <= 0 || k > len(nums) {
		return nil
	}
	var out []int
	sum := 0
	for i := 0; i < len(nums); i++ {
		sum += nums[i]
		if i >= k {
			sum -= nums[i-k]
		}
		if i >= k-1 {
			out = append(out, sum)
		}
	}
	return out
}
