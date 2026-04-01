package version

import "testing"

func TestNextTag(t *testing.T) {
	next, err := NextTag("v1.2.3", "v", "patch")
	if err != nil {
		t.Fatalf("NextTag() error = %v", err)
	}
	if next != "v1.2.4" {
		t.Fatalf("expected v1.2.4, got %s", next)
	}
}

func TestCompareTags(t *testing.T) {
	cmp, err := CompareTags("0.1.0", "v0.2.0", "v")
	if err != nil {
		t.Fatalf("CompareTags() error = %v", err)
	}
	if cmp >= 0 {
		t.Fatalf("expected 0.1.0 to be older than v0.2.0")
	}
}
