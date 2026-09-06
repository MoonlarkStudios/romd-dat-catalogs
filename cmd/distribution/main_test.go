package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	candidateArgs := []string{"candidate", "--root", filepath.Join(keys, "public", "1.root.json"), "--bundle", staged, "--cache", filepath.Join(dir, "cache"), "--catalog", "synthetic/console/standard", "--name", "ROMD Synthetic Console", "--out", filepath.Join(dir, "candidate")}
	if e = run(candidateArgs); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(filepath.Join(dir, "candidate", "candidate.dat"))
	if e != nil {
		t.Fatal(e)
	}
	original, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	if string(raw) != string(original) {
		t.Fatal("candidate changed document bytes")
	}
	if e = run(candidateArgs); e == nil {
		t.Fatal("overwrote an existing candidate")
	}
	gate, e := os.OpenFile(filepath.Join(dir, "cache", "candidate.lock"), os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer gate.Close()
	if e = syscall.Flock(int(gate.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		t.Fatal(e)
	}
	defer syscall.Flock(int(gate.Fd()), syscall.LOCK_UN)
	candidateArgs[len(candidateArgs)-1] = filepath.Join(dir, "overlapping-candidate")
	if e = run(candidateArgs); e == nil {
		t.Fatal("overlapping cache writer accepted")
	}
	if _, e = os.Stat(filepath.Join(dir, "overlapping-candidate")); !os.IsNotExist(e) {
		t.Fatal("busy reader wrote output")
	}
	for _, name := range []string{"index.json", "site/metadata/timestamp.json", "site/feed.xml", "assets/state.zip"} {
		if _, e = os.Stat(filepath.Join(staged, name)); e != nil {
			t.Fatal(e)
		}
	}
	data := filepath.Join(dir, "data")
	if e := run([]string{"export-data", "--state", state, "--out", data}); e != nil {
		t.Fatal(e)
	}
	exported, e := os.ReadFile(filepath.Join(data, "synthetic/console/standard.dat"))
	if e != nil || string(exported) != string(original) {
		t.Fatal("Git export changed complete DAT")
	}
	gitStage := filepath.Join(dir, "git-staged")
	if e := run([]string{"stage", "--state", state, "--trust", filepath.Join(keys, "public"), "--keys", filepath.Join(keys, "online.json"), "--out", gitStage, "--version", "2", "--data-repository", "example/data", "--data-commit", strings.Repeat("a", 40), "--definitions", "../../definitions"}); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"index.json", "site/metadata/timestamp.json", "site/feed.xml", "site/reference-data.json"} {
		if _, e := os.Stat(filepath.Join(gitStage, name)); e != nil {
			t.Fatal(e)
		}
	}
	referenceOut := filepath.Join(dir, "reference")
	referenceArgs := []string{"reference-data", "--root", filepath.Join(keys, "public", "1.root.json"), "--bundle", gitStage, "--cache", filepath.Join(dir, "reference-cache"), "--out", referenceOut}
	// The Pages alias is not trusted. The reader must use verified index bytes.
	if err := os.WriteFile(filepath.Join(gitStage, "site/reference-data.json"), []byte(`{"untrusted":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(referenceArgs); err != nil {
		t.Fatal(err)
	}
	reference, err := os.ReadFile(filepath.Join(referenceOut, "reference-data.json"))
	if err != nil {
		t.Fatal(err)
	}
	publication, err := os.ReadFile(filepath.Join(referenceOut, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Version     int64           `json:"version"`
		Definitions json.RawMessage `json:"definitions"`
	}
	if err := json.Unmarshal(publication, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Version != 2 || !bytes.Equal(reference, parsed.Definitions) {
		t.Fatal("reference bytes/version do not match authenticated publication")
	}
	var seeds struct {
		SchemaVersion int
		Systems       map[string]json.RawMessage
	}
	if err := json.Unmarshal(reference, &seeds); err != nil || seeds.SchemaVersion != 2 || len(seeds.Systems) != 56 {
		t.Fatal("incomplete seeds", err)
	}
	if err := run(referenceArgs); err == nil {
		t.Fatal("overwrote existing reference output")
	}
	referenceArgs[len(referenceArgs)-1] = filepath.Join(dir, "legacy-reference")
	referenceArgs[4] = staged
	referenceArgs[6] = filepath.Join(dir, "legacy-reference-cache")
	if err := run(referenceArgs); err == nil || !strings.Contains(err.Error(), "no shared reference data") {
		t.Fatal("expected a verified legacy publication without shared data", err)
	}
	if _, err := os.Stat(referenceArgs[len(referenceArgs)-1]); !os.IsNotExist(err) {
		t.Fatal("failed reference check wrote output")
	}
	if _, e := os.Stat(filepath.Join(gitStage, "assets")); !os.IsNotExist(e) {
		t.Fatal("Git CLI emitted legacy assets")
	}
}

func TestCommandValidation(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"init"}, {"stage", "--keys", "missing"}, {"restore", "--site", "http://example.invalid"}, {"reference-data"}, {"reference-data", "--site", "https://example.invalid", "--cache", "unused", "--out", "unused", "--catalog", "redump/psx/discs"}} {
		if e := run(args); e == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
