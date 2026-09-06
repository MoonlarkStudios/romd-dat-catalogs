package distribution

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

const GitFormat = "romd-signed-catalog-2"

var gitRepository = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.-]*/[A-Za-z0-9_-][A-Za-z0-9_.-]*$`)
var gitCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)
var gitCatalog = regexp.MustCompile(`^[a-z0-9]+(/[a-z0-9]+([.-][a-z0-9]+)*)+$`)
var gitAssetURL = regexp.MustCompile(`^https://raw\.githubusercontent\.com/[A-Za-z0-9_-][A-Za-z0-9_.-]*/[A-Za-z0-9_-][A-Za-z0-9_.-]*/[0-9a-f]{40}/[a-z0-9]+(/[a-z0-9]+([.-][a-z0-9]+)*)+\.dat$`)

// ExportData writes only complete current DATs to stable catalog paths. No
// timestamps or check status go in Git, so identical documents produce no diff.
func ExportData(state, checkout string) error {
	s, err := publisher.LoadSnapshot(state)
	if err != nil {
		return err
	}
	for id, c := range s.Catalogs {
		if !gitCatalog.MatchString(id) {
			return errors.New("unsupported data catalog path")
		}
		if c.Artifact == nil {
			continue
		}
		b, err := publisher.VerifiedRead(state, *c.Artifact)
		if err != nil {
			return err
		}
		if err := save(checkout, id+".dat", b); err != nil {
			return err
		}
	}
	return nil
}

// StageGit signs latest catalogs at immutable commit URLs. Git must be pushed
// and these URLs verified before Pages promotion. No DAT assets/archive emitted.
func StageGit(opt StageOptions, repository, commit string) (Index, error) {
	var result Index
	if !gitRepository.MatchString(repository) || !gitCommit.MatchString(commit) {
		return result, errors.New("data repository and full Git commit required")
	}
	if opt.Version < 1 || opt.Now.IsZero() {
		return result, errors.New("version and publication time required")
	}
	if _, err := os.Stat(opt.Output); !errors.Is(err, os.ErrNotExist) {
		return result, errors.New("staging output must not exist")
	}
	if _, ok := opt.Keys["root"]; ok {
		return result, errors.New("offline root key must not be available to publication")
	}
	root, err := readRoot(opt.TrustDir)
	if err != nil {
		return result, err
	}
	if !opt.Now.Before(root.Signed.Expires) {
		return result, errors.New("root expired; rotate offline")
	}
	if b, err := os.ReadFile(filepath.Join(opt.State, ".distribution-version")); err == nil {
		var previous int64
		if _, err = fmt.Sscan(string(b), &previous); err != nil || opt.Version <= previous {
			return result, errors.New("publication version must advance")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	s, err := publisher.LoadSnapshot(opt.State)
	if err != nil {
		return result, err
	}
	s, feed, err := publisher.Latest(s)
	if err != nil {
		return result, err
	}
	result = Index{Format: GitFormat, Version: opt.Version, Snapshot: s, Downloads: map[string]Asset{}}
	ids := make([]string, 0, len(s.Catalogs))
	for id := range s.Catalogs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		c := s.Catalogs[id]
		if !gitCatalog.MatchString(id) {
			return result, errors.New("unsupported data catalog path")
		}
		if c.Artifact == nil {
			continue
		}
		ref := c.Artifact
		// Documents shared by catalogs can share a download; their bytes are identical.
		a := Asset{"https://raw.githubusercontent.com/" + repository + "/" + commit + "/" + id + ".dat", ref.SHA256, ref.Bytes}
		if _, exists := result.Downloads[ref.Path]; !exists {
			result.Downloads[ref.Path] = a
		}
	}
	for path, a := range result.Downloads {
		feed = bytes.ReplaceAll(feed, []byte("https://catalogs.example.invalid/"+path), []byte(a.URL))
	}
	feed = bytes.ReplaceAll(feed, []byte("https://catalogs.example.invalid/"), []byte("https://github.com/"+repository+"/"))
	return signIndex(opt, result, feed)
}

// RestorePublication supports the old signed prototype once during migration;
// new publications fetch only the latest DATs referenced by the signed index.
func RestorePublication(index Index, destination string, client *http.Client) error {
	if index.Format == "romd-signed-catalog-1" {
		raw, err := FetchAsset(index.State, client)
		if err != nil {
			return err
		}
		return RestoreState(index, raw, destination)
	}
	if index.Format != GitFormat || index.Version < 1 {
		return errors.New("unsupported catalog")
	}
	docs := map[string][]byte{}
	for _, c := range index.Snapshot.Catalogs {
		if c.Artifact == nil {
			continue
		}
		ref := c.Artifact
		a, ok := index.Downloads[ref.Path]
		if !ok || a.SHA256 != ref.SHA256 || a.Bytes != ref.Bytes || !gitAssetURL.MatchString(a.URL) {
			return errors.New("invalid Git document binding")
		}
		if _, ok := docs[ref.Path]; ok {
			continue
		}
		b, err := FetchAsset(a, client)
		if err != nil {
			return err
		}
		docs[ref.Path] = b
	}
	if err := publisher.RestoreLatest(destination, index.Snapshot, docs); err != nil {
		return err
	}
	if err := save(destination, ".distribution-version", []byte(fmt.Sprint(index.Version))); err != nil {
		os.RemoveAll(destination)
		return err
	}
	return nil
}

func VerifyDownloads(index Index, client *http.Client) error {
	for _, a := range index.Downloads {
		if index.Format == GitFormat && !gitAssetURL.MatchString(a.URL) {
			return errors.New("expected immutable Git DAT URL")
		}
		if _, err := FetchAsset(a, client); err != nil {
			return err
		}
	}
	if index.Format == "romd-signed-catalog-1" {
		_, err := FetchAsset(index.State, client)
		return err
	}
	if index.Format != GitFormat {
		return errors.New("unsupported catalog")
	}
	return nil
}
