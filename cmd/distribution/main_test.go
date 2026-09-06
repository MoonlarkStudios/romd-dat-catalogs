package main

import (
	"os"
	"path/filepath"
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
}

func TestCommandValidation(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"init"}, {"stage", "--keys", "missing"}, {"restore", "--site", "http://example.invalid"}} {
		if e := run(args); e == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
