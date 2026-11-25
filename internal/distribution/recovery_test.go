package distribution

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/definitions"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

func TestRecoverPublisherRequiresAnExactVerifiedCheckpoint(t *testing.T) {
	f := makeFixture(t)
	registry, err := definitions.LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(f.dir, "data")
	if err = os.Mkdir(repo, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-b", "main")
	if err = ExportData(f.state, repo); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "--", "*.dat")
	git(t, repo, "commit", "-m", "verified publication")
	commit := git(t, repo, "rev-parse", "HEAD")
	deployed := f.now.Add(-8 * 24 * time.Hour)
	stage := filepath.Join(f.dir, "deployed")
	opt := StageOptions{State: f.state, TrustDir: f.trust, Output: stage, Version: 7, Now: deployed, Keys: f.online, Definitions: registry}
	if _, err = StageGit(opt, "example/data", commit); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(stage, "site/targets/*.catalog.json"))
	if err != nil || len(files) != 1 {
		t.Fatal("expected one signed catalog target", err)
	}
	checkpoint := PublicationCheckpoint{
		Version: 7, Repository: "example/data", Commit: commit, IndexHash: publisher.Hash(read(t, files[0])),
		VerifiedAt: deployed.Add(time.Minute),
	}
	client := func(corrupt string) *http.Client {
		return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host == "raw.githubusercontent.com" && corrupt != "dat" {
				return dataClient(repo).Transport.RoundTrip(r)
			}
			data, err := os.ReadFile(filepath.Join(stage, "site", r.URL.Path))
			if r.URL.Path == corrupt || r.URL.Host == "raw.githubusercontent.com" {
				data, err = []byte("tampered"), nil
			}
			status := http.StatusOK
			if os.IsNotExist(err) {
				status, err = http.StatusNotFound, nil
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data)), Request: r}, err
		})}
	}
	site := "https://publisher.example"
	if _, err = RefreshIndex(f.root, site, filepath.Join(f.dir, "reader-cache"), client("")); err == nil {
		t.Fatal("ordinary readers must reject the expired publication")
	}
	restored := filepath.Join(f.dir, "recovered")
	if err = RecoverPublisher(f.root, site, restored, checkpoint, client("")); err != nil {
		t.Fatal(err)
	}
	if version := string(read(t, filepath.Join(restored, ".distribution-version"))); version != "7" {
		t.Fatal("restored state lost its publication version", version)
	}
	opt.State, opt.Output, opt.Now = restored, filepath.Join(f.dir, "fresh"), f.now
	if _, err = StageGit(opt, checkpoint.Repository, commit); err == nil {
		t.Fatal("recovery must not allow publication at the same version")
	}
	opt.Version++
	if _, err = StageGit(opt, checkpoint.Repository, commit); err != nil {
		t.Fatal("recovered state could not stage the next version", err)
	}
	for _, scenario := range []struct {
		name, corrupt string
		change        func(*PublicationCheckpoint)
	}{
		{name: "version", change: func(c *PublicationCheckpoint) { c.Version++ }},
		{name: "commit", change: func(c *PublicationCheckpoint) { c.Commit = fmt.Sprintf("%040x", 1) }},
		{name: "hash", change: func(c *PublicationCheckpoint) { c.IndexHash = fmt.Sprintf("%064x", 1) }},
		{name: "expired at verification", change: func(c *PublicationCheckpoint) { c.VerifiedAt = f.now }},
		{name: "timestamp substitution", corrupt: "/metadata/timestamp.json"},
		{name: "target substitution", corrupt: "/targets/" + filepath.Base(files[0])},
		{name: "DAT substitution", corrupt: "dat"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			input := checkpoint
			if scenario.change != nil {
				scenario.change(&input)
			}
			destination := filepath.Join(t.TempDir(), "refused")
			if err := RecoverPublisher(f.root, site, destination, input, client(scenario.corrupt)); err == nil {
				t.Fatal("recovered mismatched or unauthenticated publication")
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("failed verification left a state directory", err)
			}
		})
	}
}
