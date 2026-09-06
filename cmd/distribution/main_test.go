package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

func TestInitAndStageCLI(t *testing.T) {
	dir := t.TempDir()
	keys := filepath.Join(dir, "keys")
	if e := run([]string{"init", "--out", keys}); e != nil {
		t.Fatal(e)
	}
	state := filepath.Join(dir, "state")
	file := "../../fixtures/example.dat"
	_, e := publisher.Publish(state, []publisher.Attempt{{CatalogID: "synthetic/console/standard", ExpectedName: "ROMD Synthetic Console", SourceURL: "https://example.invalid/dat", Path: &file}}, "https://catalogs.example.invalid/", publisher.Options{Now: time.Now().UTC()})
	if e != nil {
		t.Fatal(e)
	}
	staged := filepath.Join(dir, "staged")
	if e = run([]string{"stage", "--state", state, "--trust", filepath.Join(keys, "public"), "--keys", filepath.Join(keys, "online.json"), "--out", staged, "--version", "1", "--release-base", "https://github.com/o/r/releases/download/synthetic-1"}); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"index.json", "site/metadata/timestamp.json", "site/feed.xml", "assets/state.zip"} {
		if _, e = os.Stat(filepath.Join(staged, name)); e != nil {
			t.Fatal(e)
		}
	}
}

func TestCommandValidation(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"init"}, {"stage", "--keys", "missing"}, {"restore", "--site", "http://example.invalid"}} {
		if e := run(args); e == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
