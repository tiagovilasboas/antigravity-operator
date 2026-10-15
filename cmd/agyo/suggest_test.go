package main

import (
	"testing"
)

func TestSuggestCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"statsu", "session status"},
		{"status", "session status"},
		{"compact", "session compact"},
		{"compcat", "session compact"},
		{"chekpoint", "checkpoint"},
		{"rollbak", "rollback"},
		{"brwoser", "browser"},
		{"docotr", "doctor"},
		{"synk", "sync"},
		{"verison", "version"},
		{"notebuk", "notebooklm"},
		{"completelyunknownlongstring", ""},
	}

	for _, tt := range tests {
		got := suggestCommand(tt.input)
		if got != tt.expected {
			t.Errorf("suggestCommand(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestLevenshtein(t *testing.T) {
	if dist := levenshtein("kitten", "sitting"); dist != 3 {
		t.Errorf("levenshtein('kitten', 'sitting') = %d, expected 3", dist)
	}
	if dist := levenshtein("hello", "hello"); dist != 0 {
		t.Errorf("levenshtein('hello', 'hello') = %d, expected 0", dist)
	}
	if dist := levenshtein("", "abc"); dist != 3 {
		t.Errorf("levenshtein('', 'abc') = %d, expected 3", dist)
	}
}
