package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/definitions"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/redump"
	"io"
	"os"
	"path/filepath"
)

type acquirer interface {
	Acquire(context.Context, []redump.Catalog, string) ([]redump.Result, error)
}

func run(args []string, out, errOut io.Writer) error {
	return runWithAcquirer(args, out, errOut, redump.New())
}
func runWithAcquirer(args []string, out, errOut io.Writer, adapter acquirer) error {
	f := flag.NewFlagSet("publisher", flag.ContinueOnError)
	f.SetOutput(errOut)
	output := f.String("output", "", "publication directory (required)")
	paused := f.Bool("paused", false, "record selected catalog as paused without fetching (requires existing state)")
	registryPath := f.String("definitions", "definitions", "reviewed definitions directory")
	catalogID := f.String("catalog", "", "exact catalog ID to acquire; exclusive with manifest")
	base := f.String("base-url", "https://catalogs.example.invalid/", "public HTTPS base URL")
	if e := f.Parse(args); e != nil {
		return e
	}
	if *output == "" || (*catalogID == "" && (f.NArg() != 1 || *paused)) || (*catalogID != "" && f.NArg() != 0) {
		return fmt.Errorf("usage: publisher --output DIR MANIFEST | --catalog ID [--definitions DIR] [--paused]")
	}
	var a []publisher.Attempt
	var e error
	var selected definitions.Catalog
	if *catalogID != "" {
		registry, err := definitions.LoadSource(*registryPath)
		if err != nil {
			return err
		}
		var ok bool
		selected, ok = registry.Catalogs[*catalogID]
		if !ok {
			return errors.New("unknown catalog selection")
		}
		prior, err := definitions.Load(filepath.Join(*output, ".definitions.json"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := registry.Compatible(prior); err != nil {
			return err
		}
	}
	if *paused {
		prior, err := publisher.LoadSnapshot(*output)
		if err != nil {
			return err
		}
		if c, exists := prior.Catalogs[*catalogID]; exists {
			if c.Name != selected.ExpectedName {
				return errors.New("catalog identity changed; explicit migration required")
			}
			failure := "publication_paused"
			a = append(a, publisher.Attempt{CatalogID: *catalogID, ExpectedName: selected.ExpectedName, SourceURL: "http://redump.org/datfile/" + selected.ProviderSystemID + "/", Failure: &failure})
		} else {
			return writeSummary(out, prior)
		}
	} else if *catalogID != "" {
		stageRoot, err := os.MkdirTemp("", "romd-redump-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(stageRoot)
		results, err := adapter.Acquire(context.Background(), []redump.Catalog{{ID: *catalogID, System: selected.ProviderSystemID, ExpectedName: selected.ExpectedName, Platform: selected.SystemID, Representation: selected.Representation, PolicyVersion: "1", MinGames: selected.Validation.MinimumGames, MinROMs: selected.Validation.MinimumROMs}}, filepath.Join(stageRoot, "input"))
		if err != nil {
			return err
		}
		for _, result := range results {
			a = append(a, result.Attempt)
		}
	} else {
		a, e = publisher.ReadManifest(f.Arg(0))
	}
	if e != nil {
		return e
	}
	s, e := publisher.Publish(*output, a, *base, publisher.Options{})
	if e != nil {
		return e
	}
	return writeSummary(out, s)
}

func writeSummary(out io.Writer, s publisher.Snapshot) error {
	failed := 0
	for _, c := range s.Catalogs {
		if c.Health == "failed" {
			failed++
		}
	}
	return json.NewEncoder(out).Encode(map[string]any{"sequence": s.Sequence, "catalogs": len(s.Catalogs), "failed": failed, "trust": publisher.Trust, "details": s.Catalogs})
}
func main() {
	if e := run(os.Args[1:], os.Stdout, os.Stderr); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
