package textkit

import (
	"reflect"
	"testing"
)

func TestMax(t *testing.T) {
	tests := []struct {
		in   []int
		want int
	}{
		{[]int{1}, 1},
		{[]int{3, 9, 2}, 9},
		{[]int{-5, -2, -9}, -2},
		{[]int{4, 4, 1}, 4},
		{[]int{1, 2, 3, 10}, 10},
	}
	for _, tt := range tests {
		if got := Max(tt.in); got != tt.want {
			t.Errorf("Max(%v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestIsPrime(t *testing.T) {
	tests := []struct {
		in   int
		want bool
	}{
		{-7, false}, {0, false}, {1, false}, {4, false}, {9, false}, {25, false},
		{2, true}, {3, true}, {13, true}, {97, true},
	}
	for _, tt := range tests {
		if got := IsPrime(tt.in); got != tt.want {
			t.Errorf("IsPrime(%d) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello world", "Hello World"},
		{"gO pRO", "Go Pro"},
		{"a", "A"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := Capitalize(tt.in); got != tt.want {
			t.Errorf("Capitalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAtoiBase(t *testing.T) {
	tests := []struct {
		s, base string
		want    int
	}{
		{"101", "01", 5},
		{"ff", "0123456789abcdef", 255},
		{"z", "01", 0},
		{"1", "0", 0},
		{"", "01", 0},
	}
	for _, tt := range tests {
		if got := AtoiBase(tt.s, tt.base); got != tt.want {
			t.Errorf("AtoiBase(%q, %q) = %d, want %d", tt.s, tt.base, got, tt.want)
		}
	}
}

func TestWindowSums(t *testing.T) {
	tests := []struct {
		in   []int
		k    int
		want []int
	}{
		{[]int{1, 2, 3, 4}, 2, []int{3, 5, 7}},
		{[]int{5}, 1, []int{5}},
		{[]int{1, 2}, 3, nil},
		{[]int{1, 2, 3}, 0, nil},
		{[]int{1, 2, 3}, 3, []int{6}},
	}
	for _, tt := range tests {
		if got := WindowSums(tt.in, tt.k); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("WindowSums(%v, %d) = %v, want %v", tt.in, tt.k, got, tt.want)
		}
	}
}
