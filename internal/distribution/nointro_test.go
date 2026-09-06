package distribution

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/definitions"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/nointro"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

func TestSignedNoIntroCandidate(t *testing.T) {
	f := makeFixture(t)
	registry, err := definitions.LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	const id = "no-intro/snes/standard"
	c := registry.Catalogs[id]
	// Synthetic fixture floors only; production keeps the reviewed 4,000 minimum.
	c.Validation = definitions.Validation{MinimumGames: 2, MinimumROMs: 2}
	registry.Catalogs[id] = c
	path := "../../fixtures/nointro-snes.dat"
	raw := read(t, path)
	doc, counts, err := nointro.Validate(raw, c)
	if err != nil || counts.Games != 2 || counts.ROMs != 2 {
		t.Fatal("synthetic No-Intro validation", err)
	}
	if _, err := publisher.Publish(f.state, []publisher.Attempt{{CatalogID: id, ExpectedName: c.ExpectedName, SourceURL: nointro.SourceURL(c.ProviderSystemID), Path: &path}}, "https://example.invalid/", publisher.Options{Now: f.now}); err != nil {
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
	if !bytes.Equal(read(t, filepath.Join(repo, id+".dat")), doc) {
		t.Fatal("export changed bytes")
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "synthetic No-Intro fixture")
	commit := git(t, repo, "rev-parse", "HEAD")
	out := filepath.Join(f.dir, "nointro-stage")
	_, err = StageGit(StageOptions{State: f.state, TrustDir: f.trust, Output: out, Keys: f.online, Version: 1, Now: f.now, Definitions: registry}, "example/data", commit)
	if err != nil {
		t.Fatal(err)
	}
	f.location.Store(out)
	verified, err := f.refresh(filepath.Join(f.dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	candidate, received, err := ReadCandidate(verified, id, c.ExpectedName, dataClient(repo))
	if err != nil {
		t.Fatal(err)
	}
	if candidate.SystemID != "snes" || !bytes.Equal(received, doc) || candidate.SHA256 != publisher.Hash(doc) {
		t.Fatal("wrong authenticated candidate", candidate)
	}
	url := "https://raw.githubusercontent.com/example/data/" + commit + "/" + id + ".dat"
	if !bytes.Contains(read(t, filepath.Join(out, "site/feed.xml")), []byte(url)) {
		t.Fatal("RSS lacks exact immutable URL")
	}
	if err := VerifyDownloads(verified, dataClient(repo)); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(f.dir, "restored")
	if err := RestorePublication(verified, restored, dataClient(repo)); err != nil {
		t.Fatal(err)
	}
	prior, err := definitions.Load(filepath.Join(restored, ".definitions.json"))
	if err != nil || prior.Catalogs[id].ProviderSystemID != "49" {
		t.Fatal("lost restored provider mapping", err)
	}
	for _, bad := range []string{strings.Replace(url, commit, "main", 1), strings.Replace(url, "/no-intro/", "/../", 1), url + "?token=x"} {
		if gitAssetURL.MatchString(bad) {
			t.Fatal("unsafe hyphenated-provider URL allowed")
		}
	}
}
