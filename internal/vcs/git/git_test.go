package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kpango/unk/internal/vcs"
)

func TestParseNumstat(t *testing.T) {
	text := "10\t2\tsrc/main.go\x00" +
		"5\t0\tnew_file.ts\x00" +
		"0\t3\tdeleted.go\x00"
	files := vcs.ParseNumstat(text)
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	if files[0].Path != "src/main.go" || files[0].Additions != 10 || files[0].Deletions != 2 {
		t.Errorf("first file wrong: %+v", files[0])
	}
	if files[1].Path != "new_file.ts" || files[1].Additions != 5 || files[1].Deletions != 0 {
		t.Errorf("second file wrong: %+v", files[1])
	}
}

func TestParseNumstatBinary(t *testing.T) {
	text := "-\t-\tbinary.png\x00" +
		"3\t1\tnormal.go\x00"
	files := vcs.ParseNumstat(text)
	if len(files) != 1 || files[0].Path != "normal.go" {
		t.Errorf("expected only normal.go, got %v", files)
	}
}

func TestPatchToNumstat(t *testing.T) {
	patch := "diff --git a/foo.go b/foo.go\n" +
		"index abc..def 100644\n" +
		"--- a/foo.go\n" +
		"+++ b/foo.go\n" +
		"@@ -1,2 +1,3 @@\n" +
		" line1\n" +
		"+added\n" +
		"-removed\n"
	numstat := patchToNumstat(patch)
	files := vcs.ParseNumstat(numstat)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "foo.go" {
		t.Errorf("unexpected path: %s", files[0].Path)
	}
	if files[0].Additions != 1 || files[0].Deletions != 1 {
		t.Errorf("unexpected stats: %+v", files[0])
	}
}

func TestIsReviewableUntrackedPath(t *testing.T) {
	tmp := t.TempDir()

	filePath := filepath.Join(tmp, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isReviewableUntrackedPath(tmp, "hello.txt") {
		t.Error("regular file should be reviewable")
	}

	dirPath := filepath.Join(tmp, "subdir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if isReviewableUntrackedPath(tmp, "subdir") {
		t.Error("directory should not be reviewable")
	}

	if isReviewableUntrackedPath(tmp, ".unk/config") {
		t.Error(".unk/ entries should not be reviewable")
	}
}

func TestSingleFileDiff(t *testing.T) {
	patch := singleFileDiff("hello.go", "old\n", "new\n", false, false)
	if !contains(patch, "diff --git a/hello.go b/hello.go") {
		t.Errorf("missing git diff header: %s", patch)
	}
	if !contains(patch, "-old") {
		t.Errorf("missing deletion: %s", patch)
	}
	if !contains(patch, "+new") {
		t.Errorf("missing addition: %s", patch)
	}
}

func TestSingleFileDiffDeleted(t *testing.T) {
	patch := singleFileDiff("gone.go", "content\n", "", true, false)
	if !contains(patch, "deleted file mode") {
		t.Errorf("missing deleted file mode: %s", patch)
	}
}

func TestSingleFileDiffAdded(t *testing.T) {
	patch := singleFileDiff("new.go", "", "content\n", false, true)
	if !contains(patch, "new file mode") {
		t.Errorf("missing new file mode: %s", patch)
	}
}

func TestPathspecMatcher(t *testing.T) {
	m := pathspecMatcher([]string{"src/foo"})
	if !m("src/foo") {
		t.Error("exact match should pass")
	}
	if !m("src/foo/bar.go") {
		t.Error("subdirectory should pass")
	}
	if m("src/bar") {
		t.Error("unrelated path should not pass")
	}

	any := pathspecMatcher(nil)
	if !any("anything") {
		t.Error("nil pathspecs should match everything")
	}
}

func TestPathspecMatcherExclude(t *testing.T) {
	// :(exclude) pathspec: all files except the excluded one should pass.
	m := pathspecMatcher([]string{":(exclude)large.go"})
	if !m("src/main.go") {
		t.Error("non-excluded file should pass")
	}
	if !m("pkg/util.go") {
		t.Error("non-excluded file should pass")
	}
	if m("large.go") {
		t.Error("excluded file should not pass")
	}

	// Mixed: include + exclude.
	m2 := pathspecMatcher([]string{"src", ":(exclude)src/skip.go"})
	if !m2("src/main.go") {
		t.Error("included non-excluded file should pass")
	}
	if m2("src/skip.go") {
		t.Error("excluded file in included dir should not pass")
	}
	if m2("other/file.go") {
		t.Error("file outside include should not pass")
	}
}

func TestBinaryFileDiff(t *testing.T) {
	d := binaryFileDiff("image.png", false, false)
	if !contains(d, "diff --git a/image.png b/image.png") {
		t.Errorf("missing diff header: %s", d)
	}
	if !contains(d, "Binary files a/image.png and b/image.png differ") {
		t.Errorf("missing binary marker: %s", d)
	}
}

func TestBuildPatchFromContentBinary(t *testing.T) {
	// Content with null byte should produce a binary marker, not a text diff.
	binaryContent := "PNG\x00\x01\x02"
	patch := buildPatchFromContent(
		map[string]string{"image.png": binaryContent},
		map[string]string{"image.png": binaryContent + "extra"},
		nil,
	)
	if !contains(patch, "Binary files") {
		t.Errorf("expected Binary files marker, got: %q", patch)
	}
	if contains(patch, "@@") {
		t.Errorf("should not contain unk headers for binary file: %q", patch)
	}
}

func TestFilterPatchByPathspecs(t *testing.T) {
	patch := "diff --git a/foo.go b/foo.go\nsome content\n" +
		"diff --git a/bar.go b/bar.go\nother content\n"
	filtered := filterPatchByPathspecs(patch, []string{"foo.go"})
	if !contains(filtered, "foo.go") {
		t.Error("expected foo.go in filtered result")
	}
	if contains(filtered, "bar.go") {
		t.Error("bar.go should have been filtered out")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
