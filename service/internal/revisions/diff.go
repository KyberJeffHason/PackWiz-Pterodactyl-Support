package revisions

type Change struct {
	Path   string `json:"path"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}
type Diff struct {
	Added   []Change `json:"added"`
	Removed []Change `json:"removed"`
	Changed []Change `json:"changed"`
}

func Compare(before, after Manifest) Diff {
	old := map[string]string{}
	next := map[string]string{}
	for _, f := range before.Files {
		old[f.Path] = f.SHA256
	}
	for _, f := range after.Files {
		next[f.Path] = f.SHA256
	}
	var d Diff
	for path, hash := range next {
		if prior, ok := old[path]; !ok {
			d.Added = append(d.Added, Change{Path: path, After: hash})
		} else if prior != hash {
			d.Changed = append(d.Changed, Change{Path: path, Before: prior, After: hash})
		}
	}
	for path, hash := range old {
		if _, ok := next[path]; !ok {
			d.Removed = append(d.Removed, Change{Path: path, Before: hash})
		}
	}
	return d
}
