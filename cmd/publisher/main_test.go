package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	var out, errOut bytes.Buffer
	root := filepath.Join(t.TempDir(), "output")
	if e := run([]string{"--output", root, "../../fixtures/manifest.json"}, &out, &errOut); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), `"failed":0`) || !strings.Contains(out.String(), `"trust":"unsigned-development"`) {
		t.Fatal(out.String())
	}
}
func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{}, {"--output", "unused"}, {"--unknown"}} {
		var out bytes.Buffer
		if e := run(args, &out, &out); e == nil {
			t.Fatal("accepted invalid args")
		}
	}
}

// Pausing must preserve installed content and must not invent an unavailable
// source in deployments that have never opted in to PSX publication.
func TestPausedPSX(t *testing.T) {
	for _, present := range []bool{false, true} {
		t.Run(fmt.Sprint(present), func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "output")
			attempts, err := publisher.ReadManifest("../../fixtures/manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			if present {
				attempts[0].CatalogID = "redump/psx/discs"
			}
			before, err := publisher.Publish(root, attempts, "https://example.invalid/", publisher.Options{})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := run([]string{"--output", root, "--paused", "redump-psx"}, &out, &out); err != nil {
				t.Fatal(err)
			}
			after, err := publisher.LoadSnapshot(root)
			if err != nil {
				t.Fatal(err)
			}
			var report struct {
				Details map[string]publisher.Catalog `json:"details"`
			}
			if err := json.Unmarshal(out.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if !present {
				if after.Sequence != before.Sequence || len(report.Details) != 1 {
					t.Fatal("pause mutated absent source")
				}
				if _, exists := after.Catalogs["redump/psx/discs"]; exists {
					t.Fatal("invented source")
				}
				return
			}
			old, current := before.Catalogs["redump/psx/discs"], after.Catalogs["redump/psx/discs"]
			if current.Health != "failed" || current.Error == nil || *current.Error != "publication_paused" {
				t.Fatal(current)
			}
			if current.Artifact == nil || *current.Artifact != *old.Artifact || *current.LastSuccessfulCheck != *old.LastSuccessfulCheck || *current.LastChanged != *old.LastChanged {
				t.Fatal("lost working artifact or freshness history")
			}
			if report.Details["redump/psx/discs"].Health != "failed" {
				t.Fatal("missing summary health")
			}
		})
	}
}
