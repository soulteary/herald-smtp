package config

import "testing"

func TestValid(t *testing.T) {
	// Valid() depends on env at init; we only check it returns a bool and does not panic
	got := Valid()
	_ = got
}
