package definitions

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func copySource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"catalogs.json", "system-keys.json"} {
		b, err := os.ReadFile(filepath.Join("../../definitions", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
func TestSourceAssembly(t *testing.T) {
	first, err := LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	dir := copySource(t)
	// Formatting and file creation order do not alter the assembled snapshot.
	for _, name := range []string{"catalogs.json", "system-keys.json"} {
		path := filepath.Join(dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, b); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, compact.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	second, err := LoadSource(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if !bytes.Equal(a, b) {
		t.Fatal("source layout changed snapshot")
	}
	restored, err := Parse(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Compatible(restored); err != nil {
		t.Fatal("compiled snapshot does not preserve identity", err)
	}
}
func TestSourceFailures(t *testing.T) {
	cases := map[string]func(string) error{
		"missing keys":     func(d string) error { return os.Remove(filepath.Join(d, "system-keys.json")) },
		"unknown category": func(d string) error { return os.WriteFile(filepath.Join(d, "companies.json"), []byte(`{}`), 0600) },
		"duplicate key": func(d string) error {
			return os.WriteFile(filepath.Join(d, "system-keys.json"), []byte(`{"schemaVersion":1,"systems":["psx"],"systems":["psx"]}`), 0600)
		},
		"unknown system": func(d string) error {
			return os.WriteFile(filepath.Join(d, "system-keys.json"), []byte(`{"schemaVersion":1,"systems":["other"]}`), 0600)
		},
		"duplicate system": func(d string) error {
			return os.WriteFile(filepath.Join(d, "system-keys.json"), []byte(`{"schemaVersion":1,"systems":["psx","psx"]}`), 0600)
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			dir := copySource(t)
			if err := change(dir); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadSource(dir); err == nil {
				t.Fatal("bad source accepted")
			}
		})
	}
}
