package updatecheck

import "testing"

func TestSemverGT(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "0.9.9", true},
		{"0.9.9", "1.0.0", false},
		{"1.0.0", "1.0.0", false},
		{"1.1.0", "1.0.9", true},
		{"1.0.1", "1.0.0", true},
		{"0.12.0", "0.11.9", true},
		{"0.12.0-beta.2", "0.12.0-beta.1", false}, // prerelease: base only compared
	}
	for _, tt := range tests {
		got := semverGT(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("semverGT(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSelectUpdateNotice(t *testing.T) {
	tags := map[string]string{"latest": "1.2.0", "beta": "1.3.0-beta.1"}

	// stable installed, newer stable available
	notice := selectUpdateNotice("1.1.0", tags)
	if notice == "" {
		t.Error("expected notice for outdated stable version")
	}

	// stable installed, up to date
	notice = selectUpdateNotice("1.2.0", tags)
	if notice != "" {
		t.Errorf("expected no notice for current stable version, got %q", notice)
	}

	// prerelease installed, newer beta available
	notice = selectUpdateNotice("1.2.0-beta.0", tags)
	if notice == "" {
		t.Error("expected notice for outdated prerelease version")
	}

	// dev version: skip
	notice = selectUpdateNotice("dev", tags)
	if notice != "" {
		t.Errorf("expected no notice for dev version, got %q", notice)
	}
}
