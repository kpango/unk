package pager

import "testing"

func TestLooksLikePatch(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "git diff header",
			input: "diff --git a/foo.go b/foo.go\nindex abc..def 100644\n--- a/foo.go\n+++ b/foo.go\n@@ -1,3 +1,4 @@",
			want:  true,
		},
		{
			name:  "git diff header at start (no leading newline)",
			input: "diff --git a/foo.go b/foo.go\n",
			want:  true,
		},
		{
			name:  "raw unified diff starting with ---",
			input: "--- a/foo.go\n+++ b/foo.go\n@@ -1,3 +1,4 @@\n context\n",
			want:  true,
		},
		{
			name:  "unk header only",
			input: "@@ -1,5 +1,6 @@\n context line\n",
			want:  true,
		},
		{
			name:  "unk header after newlines",
			input: "\n\n@@ -1,5 +1,6 @@\n",
			want:  true,
		},
		{
			name:  "plain text",
			input: "This is just some plain text\nwith no diff markers.\n",
			want:  false,
		},
		{
			name:  "plain text with dashes but no diff format",
			input: "-- this is a SQL comment\n-- not a diff\n",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "--- without +++ is not a diff",
			input: "--- standalone\nsome other text\n",
			want:  false,
		},
	}

	for _, tt := range tests {
		got := LooksLikePatch(tt.input)
		if got != tt.want {
			t.Errorf("%s: LooksLikePatch = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHasPrefixLine(t *testing.T) {
	tests := []struct {
		s      string
		prefix string
		want   bool
	}{
		{"diff --git a b\n", "diff --git ", true},
		{"foo\ndiff --git a b\n", "diff --git ", true},
		{"foo\nbar\n", "diff --git ", false},
		{"", "diff --git ", false},
	}
	for _, tt := range tests {
		got := HasPrefixLine(tt.s, tt.prefix)
		if got != tt.want {
			t.Errorf("HasPrefixLine(%q, %q) = %v, want %v", tt.s, tt.prefix, got, tt.want)
		}
	}
}
