package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/redump"
	"io"
	"os"
	"path/filepath"
)

func run(args []string, out, errOut io.Writer) error {
	f := flag.NewFlagSet("publisher", flag.ContinueOnError)
	f.SetOutput(errOut)
	output := f.String("output", "", "publication directory (required)")
	paused := f.Bool("paused", false, "record redump-psx as paused without fetching (requires existing state)")
	base := f.String("base-url", "https://catalogs.example.invalid/", "public HTTPS base URL")
	if e := f.Parse(args); e != nil {
		return e
	}
	if f.NArg() != 1 || *output == "" || (*paused && f.Arg(0) != "redump-psx") {
		return fmt.Errorf("usage: publisher --output DIR [--base-url URL] MANIFEST_OR_redump-psx")
	}
	var a []publisher.Attempt
	var e error
	if *paused {
		prior, err := publisher.LoadSnapshot(*output)
		if err != nil {
			return err
		}
		if c, exists := prior.Catalogs["redump/psx/discs"]; exists {
			failure := "publication_paused"
			a = append(a, publisher.Attempt{CatalogID: "redump/psx/discs", ExpectedName: c.Name, SourceURL: "http://redump.org/datfile/psx/", Failure: &failure})
		} else {
			return writeSummary(out, prior)
		}
	} else if f.Arg(0) == "redump-psx" {
		stageRoot, err := os.MkdirTemp("", "romd-redump-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(stageRoot)
		results, err := redump.New().Acquire(context.Background(), []redump.Catalog{{ID: "redump/psx/discs", System: "psx", ExpectedName: "Sony - PlayStation", Platform: "psx", Representation: "discs", PolicyVersion: "1", MinGames: 10000, MinROMs: 50000}}, filepath.Join(stageRoot, "input"))
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
