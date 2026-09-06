package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/checkpoint"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/redump"
)

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("commands: init, inspect, recover")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	state := f.String("state", ".source-state/redump", "local state or remote-mode lock directory")
	repo := f.String("remote-repo", "", "GitHub owner/repo for authoritative checkpoint")
	registry := f.String("registry", "", "reviewed catalog JSON array (init only)")
	digest := f.String("expected-digest", "", "digest from a current inspect operation")
	notBefore := f.String("not-before", "", "reviewed earliest retry, RFC3339")
	stopped := f.Bool("confirm-stopped", false, "confirm the interrupted runner is stopped")
	retry := f.Bool("confirm-retry-window", false, "confirm the upstream retry window was reviewed")
	discard := f.Bool("discard-candidates", false, "discard unpublished candidates from the interrupted run")
	if e := f.Parse(args[1:]); e != nil {
		return e
	}
	if f.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	s := redump.NewScheduler(*state)
	if *repo != "" {
		store, e := checkpoint.NewGitHub(*repo, os.Getenv("GITHUB_TOKEN"))
		if e != nil {
			return e
		}
		s, e = redump.NewRemoteScheduler(*state, store)
		if e != nil {
			return e
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	switch args[0] {
	case "init":
		file, e := os.Open(*registry)
		if e != nil {
			return e
		}
		defer file.Close()
		b, e := io.ReadAll(io.LimitReader(file, (1<<20)+1))
		if e != nil {
			return e
		}
		if len(b) > 1<<20 {
			return errors.New("registry size limit")
		}
		var cs []redump.Catalog
		if e = json.Unmarshal(b, &cs); e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Dir(*state), 0700); e != nil {
			return e
		}
		return s.Initialize(cs)
	case "inspect":
		review, e := s.Inspect(ctx)
		if e != nil {
			return e
		}
		return json.NewEncoder(out).Encode(review)
	case "recover":
		next, e := time.Parse(time.RFC3339, *notBefore)
		if e != nil {
			return errors.New("valid --not-before required")
		}
		return s.Recover(ctx, redump.RecoveryOptions{ExpectedDigest: *digest, NotBefore: next, ConfirmedStopped: *stopped, ConfirmedRetryWindow: *retry, DiscardCandidates: *discard})
	default:
		return errors.New("unknown source-state command")
	}
}
func main() {
	if e := run(os.Args[1:], os.Stdout); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
