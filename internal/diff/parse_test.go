package diff

import (
	"strings"
	"testing"
)

const samplePatch = `diff --git a/src/main.go b/src/main.go
index abc123..def456 100644
--- a/src/main.go
+++ b/src/main.go
@@ -1,4 +1,6 @@
 package main

+import "fmt"
+
 func main() {
-	println("hello")
+	fmt.Println("hello")
 }
`

func TestParsePatchFiles(t *testing.T) {
	results := ParsePatchFiles(samplePatch)
	if len(results) != 1 {
		t.Fatalf("expected 1 file, got %d", len(results))
	}
	r := results[0]
	if r.Metadata.Name != "src/main.go" {
		t.Errorf("unexpected name: %q", r.Metadata.Name)
	}
	if len(r.Metadata.Unks) != 1 {
		t.Errorf("expected 1 unk, got %d", len(r.Metadata.Unks))
	}
	if r.Metadata.Unks[0].OldRange == nil || r.Metadata.Unks[0].OldRange[0] != 1 {
		t.Errorf("unexpected old range: %v", r.Metadata.Unks[0].OldRange)
	}
}

const multiFilePatch = `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,2 +1,3 @@
 x
+y
 z
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1,1 +1,1 @@
-old
+new
`

func TestParsePatchFilesMultiple(t *testing.T) {
	results := ParsePatchFiles(multiFilePatch)
	if len(results) != 2 {
		t.Fatalf("expected 2 files, got %d", len(results))
	}
	if results[0].Metadata.Name != "a.go" {
		t.Errorf("first file: %q", results[0].Metadata.Name)
	}
	if results[1].Metadata.Name != "b.go" {
		t.Errorf("second file: %q", results[1].Metadata.Name)
	}
}

func TestStripGitLogMetadata(t *testing.T) {
	input := `commit abc1234
Author: Alice <alice@example.com>
Date:   Mon Jan 1 12:00:00 2024

    Add feature

diff --git a/x.go b/x.go
--- a/x.go
+++ b/x.go
@@ -1 +1 @@
-old
+new
`
	result := StripGitLogMetadata(input)
	if strings.Contains(result, "Author:") {
		t.Errorf("git log metadata not stripped: %q", result)
	}
	if !strings.Contains(result, "diff --git") {
		t.Errorf("diff content removed unexpectedly: %q", result)
	}
}

func TestNormalizeGitPatchPrefixes(t *testing.T) {
	// mnemonic prefix: i/ c/ w/ o/
	input := "diff --git i/src/foo.go w/src/foo.go\n--- i/src/foo.go\n+++ w/src/foo.go\n"
	result := NormalizeGitPatchPrefixes(input)
	if strings.Contains(result, "i/") || strings.Contains(result, "w/") {
		t.Errorf("mnemonic prefixes not normalized: %q", result)
	}
	if !strings.Contains(result, "a/src/foo.go") || !strings.Contains(result, "b/src/foo.go") {
		t.Errorf("expected a/ b/ prefixes: %q", result)
	}
}

func TestIsProbablyBinary(t *testing.T) {
	if IsProbablyBinary([]byte("hello world\n")) {
		t.Error("text should not be binary")
	}
	// Null byte → binary.
	if !IsProbablyBinary([]byte{0x00, 0x01, 0x02}) {
		t.Error("data with null byte should be binary")
	}
}

func TestPatchLooksBinary(t *testing.T) {
	if !PatchLooksBinary("Binary files a/img.png and b/img.png differ") {
		t.Error("should detect binary patch")
	}
	if PatchLooksBinary("normal text patch") {
		t.Error("normal patch should not look binary")
	}
}

func TestNormPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a/src/main.go", "src/main.go"},
		{"b/src/main.go", "src/main.go"},
		{"src/main.go", "src/main.go"},
	}
	for _, c := range cases {
		if got := normPath(c.in); got != c.want {
			t.Errorf("normPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
