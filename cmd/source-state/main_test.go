package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/redump"
)

func TestLocalInitializationAndInspection(t *testing.T) {
	dir := t.TempDir()
	registry := filepath.Join(dir, "registry.json")
	state := filepath.Join(dir, "state")
	b, e := json.Marshal([]redump.Catalog{{ID: "redump/synthetic/discs", System: "synthetic", ExpectedName: "Synthetic", Platform: "synthetic", Representation: "discs", PolicyVersion: "1", MinGames: 1, MinROMs: 1}})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(registry, b, 0600); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	args := []string{"init", "--state", state, "--registry", registry}
	if e = run(args, &out); e != nil {
		t.Fatal(e)
	}
	if e = run(args, &out); e == nil {
		t.Fatal("reinitialized state")
	}
	if e = run([]string{"inspect", "--state", state}, &out); e != nil {
		t.Fatal(e)
	}
	var review redump.Review
	if e = json.Unmarshal(out.Bytes(), &review); e != nil || len(review.Digest) != 64 || review.InFlight {
		t.Fatal("invalid inspection", e)
	}
	if e = run([]string{"recover", "--state", state, "--not-before", "2026-12-01T00:00:00Z"}, &out); e == nil {
		t.Fatal("unreviewed recovery accepted")
	}
}
func TestCommandValidation(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	for _, args := range [][]string{nil, {"unknown"}, {"inspect", "unexpected"}, {"init"}, {"recover"}, {"inspect", "--remote-repo", "a/b"}} {
		if e := run(args, &bytes.Buffer{}); e == nil {
			t.Fatal(args)
		}
	}
}
