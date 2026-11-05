package project

import "testing"

func TestFixGoPatchVersionComparison(t *testing.T) {
	for _, tc := range []struct {
		have, want string
		less       bool
	}{
		{"1.27.3", "1.27.9", true}, {"1.27.9", "1.27.3", false}, {"1.27.3", "1.27.3", false}, {"1.27", "1.27.1", true}, {"1.27.3", "1.27", false}, {"1.27.3", "1.27.bad", true},
	} {
		if got := versionLess(tc.have, tc.want); got != tc.less {
			t.Errorf("%s < %s = %v", tc.have, tc.want, got)
		}
	}
}
