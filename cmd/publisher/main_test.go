package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/definitions"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/nointro"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/redump"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
			registryPath := t.TempDir()
			for _, name := range []string{"systems.json", "companies.json", "catalogs.json", "regions.json", "languages.json"} {
				raw, err := os.ReadFile(filepath.Join("../../definitions", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(registryPath, name), bytes.ReplaceAll(raw, []byte("Sony - PlayStation"), []byte("ROMD Synthetic Console")), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := run([]string{"--output", root, "--paused", "--catalog", "redump/psx/discs", "--definitions", registryPath}, &out, &out); err != nil {
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

type captureAcquirer struct {
	calls    int
	catalogs []redump.Catalog
}

func (a *captureAcquirer) Acquire(_ context.Context, c []redump.Catalog, _ string) ([]redump.Result, error) {
	a.calls++
	a.catalogs = c
	failure := "offline fixture"
	return []redump.Result{{Attempt: publisher.Attempt{CatalogID: c[0].ID, ExpectedName: c[0].ExpectedName, SourceURL: redump.SourceURL(c[0].System), Failure: &failure}}}, nil
}
func TestCatalogSelectionBeforeNetwork(t *testing.T) {
	adapter := &captureAcquirer{}
	root := filepath.Join(t.TempDir(), "state")
	var out bytes.Buffer
	args := []string{"--output", root, "--definitions", "../../definitions", "--catalog", "redump/psx/discs"}
	if err := runWithAcquirer(args, &out, &out, adapter); err != nil {
		t.Fatal(err)
	}
	c := adapter.catalogs[0]
	if adapter.calls != 1 || c.Platform != "psx" || c.System != "psx" || c.MinGames != 10000 || c.MinROMs != 50000 {
		t.Fatal(c)
	}
	args[len(args)-1] = "redump/missing/discs"
	if err := runWithAcquirer(args, &out, &out, adapter); err == nil {
		t.Fatal("unknown catalog accepted")
	}
	if adapter.calls != 1 {
		t.Fatal("unknown selection reached network")
	}
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, "systems.json"), []byte(`{"psx":{},"psx":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"companies.json", "catalogs.json", "regions.json", "languages.json"} {
		raw, err := os.ReadFile(filepath.Join("../../definitions", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bad, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	args[3] = bad
	args[len(args)-1] = "redump/psx/discs"
	if err := runWithAcquirer(args, &out, &out, adapter); err == nil {
		t.Fatal("invalid registry accepted")
	}
	if adapter.calls != 1 {
		t.Fatal("bad data reached network")
	}
}

// A fresh CLI process uses the restored signed snapshot, not adapter memory.
func TestPublishedRetryGuidancePreventsNextAcquisition(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	retry := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339Nano)
	failure := "rate_limited"
	before, err := publisher.Publish(root, []publisher.Attempt{{CatalogID: "redump/psx/discs", ExpectedName: "Sony - PlayStation", SourceURL: redump.SourceURL("psx"), Failure: &failure, RetryAt: &retry}}, "https://example.invalid/", publisher.Options{})
	if err != nil {
		t.Fatal(err)
	}
	adapter := &captureAcquirer{}
	var out bytes.Buffer
	if err := runWithAcquirer([]string{"--output", root, "--definitions", "../../definitions", "--catalog", "redump/psx/discs"}, &out, &out, adapter); err != nil {
		t.Fatal(err)
	}
	after, err := publisher.LoadSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if adapter.calls != 0 || after.Sequence != before.Sequence || *after.Catalogs["redump/psx/discs"].RetryAt != retry {
		t.Fatal("retry guidance lost across invocation")
	}
}

type captureNoIntro struct {
	calls    int
	id       string
	selected definitions.Catalog
}

func (a *captureNoIntro) Acquire(_ context.Context, id string, c definitions.Catalog, _ string) (nointro.Result, error) {
	a.calls++
	a.id = id
	a.selected = c
	failure := "synthetic failure"
	return nointro.Result{Attempt: publisher.Attempt{CatalogID: id, ExpectedName: c.ExpectedName, SourceURL: nointro.SourceURL(c.ProviderSystemID), Failure: &failure}}, nil
}
func TestNoIntroSelectionPauseAndRetry(t *testing.T) {
	redumpAdapter := &captureAcquirer{}
	noIntro := &captureNoIntro{}
	state := filepath.Join(t.TempDir(), "state")
	var out bytes.Buffer
	args := []string{"--output", state, "--definitions", "../../definitions", "--catalog", "no-intro/snes/standard"}
	if err := runWithAdapters(args, &out, &out, redumpAdapter, noIntro); err != nil {
		t.Fatal(err)
	}
	if redumpAdapter.calls != 0 || noIntro.calls != 1 || noIntro.selected.SystemID != "snes" || noIntro.selected.ProviderSystemID != "49" || noIntro.selected.Validation.MinimumGames != 4000 {
		t.Fatal("wrong provider/system route")
	}
	if err := runWithAdapters(append(args, "--paused"), &out, &out, redumpAdapter, noIntro); err != nil {
		t.Fatal(err)
	}
	snapshot, err := publisher.LoadSnapshot(state)
	if err != nil {
		t.Fatal(err)
	}
	if noIntro.calls != 1 || *snapshot.Catalogs[noIntro.id].Error != "publication_paused" {
		t.Fatal("pause fetched or wrong status")
	}
	retry := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339Nano)
	failure := "rate_limited"
	if _, err := publisher.Publish(state, []publisher.Attempt{{CatalogID: noIntro.id, ExpectedName: noIntro.selected.ExpectedName, SourceURL: nointro.SourceURL("49"), Failure: &failure, RetryAt: &retry}}, "https://example.invalid/", publisher.Options{}); err != nil {
		t.Fatal(err)
	}
	if err := runWithAdapters(args, &out, &out, redumpAdapter, noIntro); err != nil {
		t.Fatal(err)
	}
	if noIntro.calls != 1 {
		t.Fatal("restored retry deadline ignored")
	}
}
