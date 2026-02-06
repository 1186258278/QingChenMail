package api

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1, v2   string
		expected int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.1", "v1.0.0", 1},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.2.0", "v1.1.9", 1},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.2.4", "v1.2.3", 1},
		// Pre-release versions
		{"v1.3.0", "v1.3.0-beta1", 1},       // release > pre-release
		{"v1.3.0-beta1", "v1.3.0", -1},       // pre-release < release
		{"v1.3.0-beta2", "v1.3.0-beta1", 1},  // beta2 > beta1
		{"v1.3.0-rc1", "v1.3.0-beta1", 1},    // rc > beta (lexical)
		{"v1.3.0-beta1", "v1.3.0-beta1", 0},  // same
	}

	for _, tt := range tests {
		result := compareVersions(tt.v1, tt.v2)
		if result != tt.expected {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, result, tt.expected)
		}
	}
}

func TestSplitPreRelease(t *testing.T) {
	tests := []struct {
		input   string
		base    string
		preRel  string
	}{
		{"1.2.3", "1.2.3", ""},
		{"1.2.3-beta1", "1.2.3", "beta1"},
		{"1.2.3-rc.1", "1.2.3", "rc.1"},
	}

	for _, tt := range tests {
		base, pre := splitPreRelease(tt.input)
		if base != tt.base || pre != tt.preRel {
			t.Errorf("splitPreRelease(%q) = (%q, %q), want (%q, %q)", tt.input, base, pre, tt.base, tt.preRel)
		}
	}
}
