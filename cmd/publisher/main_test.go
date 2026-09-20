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
			for _, name := range []string{"system-keys.json", "catalogs.json"} {
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
	var out, diagnostics bytes.Buffer
	args := []string{"--output", root, "--definitions", "../../definitions", "--catalog", "redump/psx/discs"}
	if err := runWithAcquirer(args, &out, &diagnostics, adapter); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagnostics.String(), "catalog redump/psx/discs: offline fixture") || !json.Valid(out.Bytes()) {
		t.Fatal("diagnostics must be separate from JSON output", diagnostics.String(), out.String())
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
	if err := os.WriteFile(filepath.Join(bad, "system-keys.json"), []byte(`{"schemaVersion":1,"systems":["psx","psx"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"catalogs.json"} {
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

type batchNoIntro struct {
	ids   []string
	retry string
	crash bool
}

func (a *batchNoIntro) Acquire(_ context.Context, id string, c definitions.Catalog, stage string) (nointro.Result, error) {
	a.ids = append(a.ids, id)
	if a.crash && len(a.ids) == 2 {
		return nointro.Result{}, fmt.Errorf("staging unavailable")
	}
	result := nointro.Result{Attempt: publisher.Attempt{CatalogID: id, ExpectedName: c.ExpectedName, SourceURL: sourceURL(c)}}
	if a.retry != "" {
		failure := "provider_backoff"
		result.Attempt.Failure = &failure
		result.Attempt.RetryAt = &a.retry
	} else {
		raw, err := os.ReadFile("../../fixtures/example.dat")
		if err != nil {
			return result, err
		}
		if err := os.Mkdir(stage, 0700); err != nil {
			return result, err
		}
		path := filepath.Join(stage, "fixture.dat")
		if err := os.WriteFile(path, bytes.ReplaceAll(raw, []byte("ROMD Synthetic Console"), []byte(c.ExpectedName)), 0600); err != nil {
			return result, err
		}
		result.Attempt.Path = &path
	}
	return result, nil
}

func batchDefinitions(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	keys, err := os.ReadFile("../../definitions/system-keys.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "system-keys.json"), keys, 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := definitions.LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	for i, key := range []string{"gb", "gbc", "gba"} {
		c := registry.Catalogs["no-intro/snes/standard"]
		c.SystemID = key
		c.ProviderSystemID = fmt.Sprint(100 + i)
		c.ExpectedName = "Synthetic " + key
		registry.Catalogs["no-intro/"+key+"/standard"] = c
	}
	raw, err := json.Marshal(registry.Catalogs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "catalogs.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBatchRoutingBackoffAndRetention(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	var out bytes.Buffer
	args := []string{"--output", root, "--definitions", batchDefinitions(t), "--catalog", "no-intro/gb/standard", "--catalog", "no-intro/gbc/standard", "--catalog", "no-intro/gba/standard"}
	adapter := &batchNoIntro{}
	if err := runWithAdapters(args, &out, &out, &captureAcquirer{}, adapter); err != nil {
		t.Fatal(err)
	}
	before, err := publisher.LoadSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(adapter.ids) != 3 || len(before.Catalogs) != 3 {
		t.Fatal("batch routing incomplete")
	}
	for _, id := range adapter.ids {
		if before.Catalogs[id].Artifact == nil {
			t.Fatal("missing document", id)
		}
	}
	adapter = &batchNoIntro{retry: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339Nano)}
	if err := runWithAdapters(args, &out, &out, &captureAcquirer{}, adapter); err != nil {
		t.Fatal(err)
	}
	after, err := publisher.LoadSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(adapter.ids) != 1 {
		t.Fatal("requested provider after backoff")
	}
	for id, c := range after.Catalogs {
		if c.RetryAt == nil || *c.RetryAt != adapter.retry || *c.Artifact != *before.Catalogs[id].Artifact {
			t.Fatal("lost deadline or working catalog", id)
		}
	}
	// A fresh process selecting only another catalog must still honor cooldown.
	next := &batchNoIntro{}
	if err := runWithAdapters(args[:6], &out, &out, &captureAcquirer{}, next); err != nil {
		t.Fatal(err)
	}
	if len(next.ids) != 0 {
		t.Fatal("forgot provider cooldown across processes")
	}
	// Pausing must not erase provider cooldown.
	if err := runWithAdapters(append(args[:6:6], "--paused"), &out, &out, &captureAcquirer{}, next); err != nil {
		t.Fatal(err)
	}
	paused, err := publisher.LoadSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if paused.Catalogs["no-intro/gb/standard"].RetryAt == nil {
		t.Fatal("pause erased cooldown")
	}
}

func TestBatchPreflightAndAtomicFailure(t *testing.T) {
	for _, tail := range [][]string{
		{"--catalog", "no-intro/gb/standard", "--catalog", "missing"},
		{"--catalog", "no-intro/gb/standard", "--pause-catalog", "no-intro/gb/standard"},
	} {
		adapter := &batchNoIntro{}
		args := append([]string{"--output", filepath.Join(t.TempDir(), "state"), "--definitions", batchDefinitions(t)}, tail...)
		var out bytes.Buffer
		if err := runWithAdapters(args, &out, &out, &captureAcquirer{}, adapter); err == nil || len(adapter.ids) != 0 {
			t.Fatal("invalid selection reached acquisition")
		}
	}
	root := filepath.Join(t.TempDir(), "state")
	adapter := &batchNoIntro{crash: true}
	var out bytes.Buffer
	args := []string{"--output", root, "--definitions", batchDefinitions(t), "--catalog", "no-intro/gb/standard", "--catalog", "no-intro/gbc/standard"}
	if err := runWithAdapters(args, &out, &out, &captureAcquirer{}, adapter); err == nil {
		t.Fatal("ignored staging error")
	}
	if _, err := os.Stat(filepath.Join(root, "current.json")); !os.IsNotExist(err) {
		t.Fatal("partially published failed batch")
	}
}

func TestUnselectedCatalogCooldownDoesNotBlockOtherProvider(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	retry := time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	failure := "provider_backoff"
	_, err := publisher.Publish(root, []publisher.Attempt{{CatalogID: "no-intro/snes/standard", ExpectedName: "Nintendo - Super Nintendo Entertainment System", SourceURL: nointro.SourceURL("49"), Failure: &failure, RetryAt: &retry}}, "https://example.invalid/", publisher.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ni := &batchNoIntro{}
	rd := &captureAcquirer{}
	args := []string{"--output", root, "--definitions", batchDefinitions(t), "--catalog", "no-intro/gb/standard", "--catalog", "redump/psx/discs"}
	if err := runWithAdapters(args, &out, &out, rd, ni); err != nil {
		t.Fatal(err)
	}
	if len(ni.ids) != 0 || rd.calls != 1 {
		t.Fatal("provider-wide cooldown was ignored or leaked to Redump")
	}
	state, err := publisher.LoadSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	c := state.Catalogs["no-intro/gb/standard"]
	if c.RetryAt == nil || *c.RetryAt != retry || c.Artifact != nil {
		t.Fatal("new selection lost cooldown or invented content")
	}
}

func TestEntireConfiguredRegistry(t *testing.T) {
	registry, err := definitions.LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	for _, paused := range []bool{false, true} {
		t.Run(fmt.Sprint("paused=", paused), func(t *testing.T) {
			redumpAdapter, noIntro := &captureAcquirer{}, &captureNoIntro{}
			state := filepath.Join(t.TempDir(), "state")
			args := []string{"--output", state, "--definitions", "../../definitions"}
			if paused {
				args = append(args, "--paused")
			}
			redumpCount, noIntroCount := 0, 0
			for id, c := range registry.Catalogs {
				args = append(args, "--catalog", id)
				if c.Provider == "redump" {
					redumpCount++
				} else {
					noIntroCount++
				}
			}
			var out, diagnostics bytes.Buffer
			if err := runWithAdapters(args, &out, &diagnostics, redumpAdapter, noIntro); err != nil {
				t.Fatal(err)
			}
			if paused {
				if redumpAdapter.calls != 0 || noIntro.calls != 0 {
					t.Fatal("paused registry made requests")
				}
			} else {
				if redumpAdapter.calls != redumpCount || noIntro.calls != noIntroCount {
					t.Fatal("incomplete provider routing")
				}
				snapshot, err := publisher.LoadSnapshot(state)
				if err != nil {
					t.Fatal(err)
				}
				if len(snapshot.Catalogs) != len(registry.Catalogs) {
					t.Fatal("incomplete full-registry publication")
				}
			}
		})
	}
}

func TestCombinedSelectionLimitBeforeNetwork(t *testing.T) {
	args := []string{"--output", filepath.Join(t.TempDir(), "state"), "--definitions", "missing"}
	for i := 0; i < MaxSelectedCatalogs+1; i++ {
		flag := "--catalog"
		if i%2 == 0 {
			flag = "--pause-catalog"
		}
		args = append(args, flag, fmt.Sprintf("redump/system%d/discs", i))
	}
	redumpAdapter, noIntro := &captureAcquirer{}, &captureNoIntro{}
	var out bytes.Buffer
	err := runWithAdapters(args, &out, &out, redumpAdapter, noIntro)
	if err == nil || !strings.Contains(err.Error(), "at most 32 catalog selections") || redumpAdapter.calls != 0 || noIntro.calls != 0 {
		t.Fatal("selection bound not enforced before I/O", err)
	}
}
