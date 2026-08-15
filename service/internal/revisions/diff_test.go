package revisions

import "testing"

func TestCompare(t *testing.T) {
	d := Compare(Manifest{Files: []File{{Path: "gone", SHA256: "a"}, {Path: "changed", SHA256: "a"}}}, Manifest{Files: []File{{Path: "new", SHA256: "b"}, {Path: "changed", SHA256: "b"}}})
	if len(d.Added) != 1 || len(d.Removed) != 1 || len(d.Changed) != 1 {
		t.Fatalf("%+v", d)
	}
}
