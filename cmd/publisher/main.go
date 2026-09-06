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
	base := f.String("base-url", "https://catalogs.example.invalid/", "public HTTPS base URL")
	if e := f.Parse(args); e != nil {
		return e
	}
	if f.NArg() != 1 || *output == "" {
		return fmt.Errorf("usage: publisher --output DIR [--base-url URL] MANIFEST_OR_redump-psx")
	}
	var a []publisher.Attempt
	var e error
	if f.Arg(0) == "redump-psx" {
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
	failed := 0
	for _, c := range s.Catalogs {
		if c.Health == "failed" {
			failed++
		}
	}
	return json.NewEncoder(out).Encode(map[string]any{"sequence": s.Sequence, "catalogs": len(s.Catalogs), "failed": failed, "trust": publisher.Trust})
}
func main() {
	if e := run(os.Args[1:], os.Stdout, os.Stderr); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
