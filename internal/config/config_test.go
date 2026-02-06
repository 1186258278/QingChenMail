package config

import (
	"testing"
)

func TestGenerateRandomKey(t *testing.T) {
	key1 := generateRandomKey(32)
	key2 := generateRandomKey(32)

	if key1 == key2 {
		t.Fatal("Two generated keys should differ")
	}

	if len(key1) < 32 {
		t.Fatalf("Key too short: %d", len(key1))
	}
}
