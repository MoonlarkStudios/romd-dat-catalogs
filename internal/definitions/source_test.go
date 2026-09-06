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
	for _, name := range []string{"catalogs.json", "companies.json", "systems.json", "regions.json", "languages.json"} {
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
	for _, name := range []string{"systems.json", "companies.json", "catalogs.json", "regions.json", "languages.json"} {
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
		"missing category": func(d string) error { return os.Remove(filepath.Join(d, "companies.json")) },
		"unknown category": func(d string) error { return os.WriteFile(filepath.Join(d, "unknown.json"), []byte(`{}`), 0600) },
		"duplicate key": func(d string) error {
			return os.WriteFile(filepath.Join(d, "systems.json"), []byte(`{"psx":{"name":"Sony PlayStation","manufacturerIds":["sony"]},"psx":{"name":"Sony PlayStation","manufacturerIds":["sony"]}}`), 0600)
		},
		"dangling system": func(d string) error {
			return os.WriteFile(filepath.Join(d, "systems.json"), []byte(`{"other":{"name":"Other","manufacturerIds":[]}}`), 0600)
		},
		"dangling company": func(d string) error {
			return os.WriteFile(filepath.Join(d, "companies.json"), []byte(`{"other":{"name":"Other","aliases":[]}}`), 0600)
		},
		"null category":  func(d string) error { return os.WriteFile(filepath.Join(d, "companies.json"), []byte(`null`), 0600) },
		"array category": func(d string) error { return os.WriteFile(filepath.Join(d, "companies.json"), []byte(`[]`), 0600) },
		"invalid UTF8": func(d string) error {
			return os.WriteFile(filepath.Join(d, "companies.json"), []byte{'{', '"', 0xff, '"', ':', '{', '}', '}'}, 0600)
		},
		"oversized": func(d string) error {
			return os.WriteFile(filepath.Join(d, "companies.json"), bytes.Repeat([]byte(" "), MaxBytes+1), 0600)
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
