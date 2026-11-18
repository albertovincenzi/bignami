package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// recents remembers the last value typed for each prompt, so you do not retype the same bucket or queue.
// It is a convenience: a read or write error is ignored on purpose (shortcut: no cache, nothing breaks).
type recents struct {
	path string
	vals map[string]string
}

func loadRecents() *recents {
	r := &recents{vals: map[string]string{}}
	dir, err := os.UserConfigDir()
	if err != nil {
		return r
	}
	r.path = filepath.Join(dir, "bignami", "recent.json")
	if b, err := os.ReadFile(r.path); err == nil {
		_ = json.Unmarshal(b, &r.vals)
	}
	return r
}

func (r *recents) get(label string) string {
	if r == nil {
		return ""
	}
	return r.vals[label]
}

func (r *recents) set(label, value string) {
	if r == nil || r.path == "" || r.vals[label] == value {
		return
	}
	r.vals[label] = value
	b, err := json.Marshal(r.vals)
	if err != nil || os.MkdirAll(filepath.Dir(r.path), 0o700) != nil {
		return
	}
	_ = os.WriteFile(r.path, b, 0o600)
}
