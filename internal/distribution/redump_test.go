package distribution

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/definitions"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/redump"
)

func TestSignedRedumpCandidate(t *testing.T) {
	for _, system := range []string{"psx", "saturn", "segacd", "dc"} {
		t.Run(system, func(t *testing.T) { testSignedRedumpCandidate(t, system) })
	}
}
func testSignedRedumpCandidate(t *testing.T, system string) {
	f := makeFixture(t)
	registry, err := definitions.LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	id := "redump/" + system + "/discs"
	c := registry.Catalogs[id]
	// Synthetic fixture floors only; production keeps measured per-system floors.
	c.Validation = definitions.Validation{MinimumGames: 2, MinimumROMs: 2}
	registry.Catalogs[id] = c
	raw := []byte(`<datafile><header><name>` + strings.ReplaceAll(c.ExpectedName, "&", "&amp;") + `</name><version>synthetic</version></header><game name="Example (Disc 1)"><rom name="disc1.cue" size="1" crc="12345678"/><rom name="disc1 (Track 1).bin" size="2352" crc="87654321"/></game><game name="Example (Disc 2)"><rom name="disc2.bin" size="2352" crc="11223344"/></game></datafile>`)
	path := filepath.Join(t.TempDir(), "synthetic.dat")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	doc, counts, err := publisher.Document(raw, c.ExpectedName)
	if err != nil || counts.Games != 2 || counts.ROMs != 3 {
		t.Fatal("synthetic Redump validation", err)
	}
	if _, err := publisher.Publish(f.state, []publisher.Attempt{{CatalogID: id, ExpectedName: c.ExpectedName, SourceURL: redump.SourceURL(c.ProviderSystemID), Path: &path}}, "https://example.invalid/", publisher.Options{Now: f.now}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := publisher.Document(raw, "Wrong System"); err == nil {
		t.Fatal("wrong system header accepted")
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
	git(t, repo, "commit", "-m", "synthetic Redump fixture")
	commit := git(t, repo, "rev-parse", "HEAD")
	out := filepath.Join(f.dir, "redump-stage")
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
	if candidate.SystemID != system || !bytes.Equal(received, doc) || candidate.SHA256 != publisher.Hash(doc) {
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
	if err != nil || prior.Catalogs[id].ProviderSystemID != c.ProviderSystemID {
		t.Fatal("lost restored provider mapping", err)
	}
	for _, bad := range []string{strings.Replace(url, commit, "main", 1), strings.Replace(url, "/redump/", "/../", 1), url + "?token=x"} {
		if gitAssetURL.MatchString(bad) {
			t.Fatal("unsafe hyphenated-provider URL allowed")
		}
	}
}
