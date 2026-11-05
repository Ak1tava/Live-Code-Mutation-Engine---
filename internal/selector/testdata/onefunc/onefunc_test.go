package onefunc

import "testing"

func TestClamp(t *testing.T) {
	for _, c := range [][4]int{{5, 0, 9, 5}, {-1, 0, 9, 0}, {12, 0, 9, 9}, {0, 0, 9, 0}, {9, 0, 9, 9}} {
		if got := Clamp(c[0], c[1], c[2]); got != c[3] {
			t.Errorf("Clamp(%d,%d,%d) = %d, want %d", c[0], c[1], c[2], got, c[3])
		}
	}
}
