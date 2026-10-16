package main

import (
	"reflect"
	"testing"
)

// TestQA_PermuteArgs_EdgeCases thoroughly audits the argument permutation engine
// ensuring proper ordering between positional arguments, valued flags, and boolean flags.
func TestQA_PermuteArgs_EdgeCases(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		boolFlags map[string]bool
		expected  []string
	}{
		{
			name:      "empty input",
			args:      []string{},
			boolFlags: nil,
			expected:  []string{},
		},
		{
			name:      "only positional arguments",
			args:      []string{"alpha", "beta", "gamma"},
			boolFlags: nil,
			expected:  []string{"alpha", "beta", "gamma"},
		},
		{
			name:      "only flags without positional",
			args:      []string{"--foo", "bar", "--baz=123"},
			boolFlags: nil,
			expected:  []string{"--foo", "bar", "--baz=123"},
		},
		{
			name:      "single dash stdin indicator preserved as positional",
			args:      []string{"-", "--notebook", "nb1"},
			boolFlags: nil,
			expected:  []string{"--notebook", "nb1", "-"},
		},
		{
			name:      "mixed interleaved flags and positional arguments",
			args:      []string{"target1", "--port", "9222", "target2", "--out", "dist", "target3"},
			boolFlags: nil,
			expected:  []string{"--port", "9222", "--out", "dist", "target1", "target2", "target3"},
		},
		{
			name:      "boolean flags do not consume following positional arguments",
			args:      []string{"my-doc-id", "--json", "extra-positional"},
			boolFlags: map[string]bool{"json": true},
			expected:  []string{"--json", "my-doc-id", "extra-positional"},
		},
		{
			name:      "boolean flags with equal signs",
			args:      []string{"pos", "--json=true", "--verbose=false"},
			boolFlags: map[string]bool{"json": true, "verbose": true},
			expected:  []string{"--json=true", "--verbose=false", "pos"},
		},
		{
			name:      "help flags (-h, --help, help) handled without consuming args",
			args:      []string{"subcmd", "-h", "param"},
			boolFlags: nil,
			expected:  []string{"-h", "subcmd", "param"},
		},
		{
			name:      "double dash terminates flag reordering completely",
			args:      []string{"cmd", "--notebook", "123", "--", "pos1", "--not-a-flag", "val"},
			boolFlags: nil,
			expected:  []string{"--notebook", "123", "cmd", "--", "pos1", "--not-a-flag", "val"},
		},
		{
			name:      "flag at end of arguments without value",
			args:      []string{"url", "--notebook"},
			boolFlags: nil,
			expected:  []string{"--notebook", "url"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := permuteArgs(tc.args, tc.boolFlags)
			if len(got) == 0 && len(tc.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("permuteArgs mismatch:\n got:  %v\n want: %v", got, tc.expected)
			}
		})
	}
}
