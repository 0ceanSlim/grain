package config

import "testing"

func TestIsDMProtectedKind(t *testing.T) {
	if !IsDMProtectedKind(1059) {
		t.Error("kind 1059 (gift wrap) should be protected")
	}
	for _, k := range []int{0, 1, 3, 4, 13, 14, 15, 10050, 30000} {
		if IsDMProtectedKind(k) {
			t.Errorf("kind %d should not be protected", k)
		}
	}
}
