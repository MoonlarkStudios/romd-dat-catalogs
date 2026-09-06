package distribution

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/definitions"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

func TestSignedSharedDefinitions(t *testing.T) {
	f := makeFixture(t)
	registry, err := definitions.LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	c := registry.Catalogs["redump/psx/discs"]
	c.ExpectedName = "ROMD Synthetic Console"
	c.Validation = definitions.Validation{MinimumGames: 1, MinimumROMs: 1}
	registry.Catalogs["redump/psx/discs"] = c
	file := "../../fixtures/example.dat"
	if _, err := publisher.Publish(f.state, []publisher.Attempt{{CatalogID: "redump/psx/discs", ExpectedName: c.ExpectedName, SourceURL: "http://redump.org/datfile/psx/", Path: &file}}, "https://catalogs.example.invalid/", publisher.Options{}); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(f.dir, "data")
	if err := os.Mkdir(repo, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-b", "main")
	if err := ExportData(f.state, repo); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "fixture")
	out := filepath.Join(f.dir, "signed-definitions")
	opt := StageOptions{State: f.state, TrustDir: f.trust, Output: out, Keys: f.online, Version: 1, Now: f.now, Definitions: registry}
	index, err := StageGit(opt, "example/data", git(t, repo, "rev-parse", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	f.location.Store(out)
	verified, err := f.refresh(filepath.Join(f.dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	if verified.Definitions == nil || verified.Definitions.Catalogs["redump/psx/discs"].SystemID != "psx" {
		t.Fatal("missing signed system identity")
	}
	if verified.Definitions.Companies["sony"].Name != "Sony" || len(verified.Definitions.Companies) != 15 || verified.Definitions.Systems["psx"].ManufacturerIDs[0] != "sony" {
		t.Fatal("company grouping missing from signed index")
	}
	expected, _ := json.Marshal(index.Definitions)
	if !bytes.Equal(expected, read(t, filepath.Join(out, "site/reference-data.json"))) {
		t.Fatal("alias differs from signed index definitions")
	}
	if !bytes.Equal(expected, read(t, filepath.Join(out, "site/targets", publisher.Hash(expected)+".reference-data.json"))) {
		t.Fatal("systems target differs")
	}
	var targets struct {
		Signed struct {
			Targets map[string]struct {
				Length int               `json:"length"`
				Hashes map[string]string `json:"hashes"`
			} `json:"targets"`
		} `json:"signed"`
	}
	if err := json.Unmarshal(read(t, filepath.Join(out, "site/metadata/1.targets.json")), &targets); err != nil {
		t.Fatal(err)
	}
	if _, exists := targets.Signed.Targets["systems.json"]; exists {
		t.Fatal("old partial-sounding target name remains")
	}
	if _, err := os.Stat(filepath.Join(out, "site/systems.json")); !os.IsNotExist(err) {
		t.Fatal("obsolete systems alias emitted")
	}
	target, ok := targets.Signed.Targets["reference-data.json"]
	if !ok || target.Length != len(expected) || target.Hashes["sha256"] != publisher.Hash(expected) {
		t.Fatal("definitions not authenticated as a TUF target")
	}
	candidate, _, err := ReadCandidate(verified, "redump/psx/discs", c.ExpectedName, dataClient(repo))
	if err != nil || candidate.SystemID != "psx" {
		t.Fatalf("system-bound candidate: %+v %v", candidate, err)
	}
	restored := filepath.Join(f.dir, "restored")
	if err := RestorePublication(verified, restored, dataClient(repo)); err != nil {
		t.Fatal(err)
	}
	prior, err := definitions.Load(filepath.Join(restored, ".definitions.json"))
	if err != nil {
		t.Fatal(err)
	}
	if prior.Companies["sony"].Name != "Sony" || prior.Systems["psx"].ManufacturerIDs[0] != "sony" {
		t.Fatal("company identity lost on restore")
	}
	if err := registry.Compatible(prior); err != nil {
		t.Fatal(err)
	}
	opt.State = restored
	opt.Output = filepath.Join(f.dir, "changed-binding")
	opt.Version = 2
	changed := registry.Catalogs["redump/psx/discs"]
	changed.ProviderSystemID = "different"
	registry.Catalogs["redump/psx/discs"] = changed
	if _, err := StageGit(opt, "example/data", git(t, repo, "rev-parse", "HEAD")); err == nil {
		t.Fatal("published reassigned catalog identity")
	}
	opt.Definitions = nil
	opt.Output = filepath.Join(f.dir, "dropped-definitions")
	if _, err := StageGit(opt, "example/data", git(t, repo, "rev-parse", "HEAD")); err == nil {
		t.Fatal("silently dropped definitions")
	}
	// A legacy publication without definitions remains readable, but has no inferred system ID.
	_, legacy := f.stage(t, 3)
	legacyIndex, err := RefreshIndex(f.root, BundleSite, filepath.Join(f.dir, "legacy-cache"), BundleClient(legacy))
	if err != nil {
		t.Fatal(err)
	}
	info, _, err := ReadCandidate(legacyIndex, "synthetic/console/standard", c.ExpectedName, BundleClient(legacy))
	if err != nil || info.SystemID != "" {
		t.Fatal("legacy reader compatibility changed", err)
	}
}
