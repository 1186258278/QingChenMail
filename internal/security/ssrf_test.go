package security

import (
	"testing"
)

func TestIsInternalURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"http://localhost/test", true},
		{"http://127.0.0.1/test", true},
		{"http://192.168.1.1/test", true},
		{"http://10.0.0.1/test", true},
		{"http://172.16.0.1/test", true},
		{"http://[::1]/test", true},
		{"not-a-url", true},                     // parse failure = blocked
		{"http://github.com/test", false},        // public URL
		{"https://api.github.com/repos", false},  // public URL
	}

	for _, tt := range tests {
		result := IsInternalURL(tt.url)
		if result != tt.expected {
			t.Errorf("IsInternalURL(%q) = %v, want %v", tt.url, result, tt.expected)
		}
	}
}
