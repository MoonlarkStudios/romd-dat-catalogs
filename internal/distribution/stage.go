package distribution

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

const MaxState = 64 << 20

var releaseURL = regexp.MustCompile(`^https://github.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/releases/download/[A-Za-z0-9_.-]+/?$`)

type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}
type Index struct {
	Format    string             `json:"format"`
	Version   int64              `json:"version"`
	Snapshot  publisher.Snapshot `json:"snapshot"`
	State     Asset              `json:"state"`
	Downloads map[string]Asset   `json:"downloads"`
}
type StageOptions struct {
	State, TrustDir, Output, ReleaseBase string
	Keys                                 Keys
	Version                              int64
	Now                                  time.Time
}

func save(dir, name string, b []byte) error {
	p := filepath.Join(dir, name)
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		return e
	}
	return os.WriteFile(p, b, 0644)
}
func asset(url string, b []byte) Asset { return Asset{url, publisher.Hash(b), len(b)} }
func metaFile(version int64, b []byte) *metadata.MetaFiles {
	h := sha256.Sum256(b)
	m := metadata.MetaFile(version)
	m.Length = int64(len(b))
	m.Hashes = metadata.Hashes{"sha256": h[:]}
	return m
}

// Stage writes only to a new private staging directory. It never advances a live
// timestamp or changes the input publisher state. Remote promotion is a later step.
func Stage(opt StageOptions) (Index, error) {
	var result Index
	if opt.Version < 1 || opt.Now.IsZero() {
		return result, errors.New("version and publication time required")
	}
	if !releaseURL.MatchString(opt.ReleaseBase) {
		return result, errors.New("expected GitHub release download URL")
	}
	if _, e := os.Stat(opt.Output); !errors.Is(e, os.ErrNotExist) {
		return result, errors.New("staging output must not exist")
	}
	if _, ok := opt.Keys["root"]; ok {
		return result, errors.New("offline root key must not be available to publication")
	}
	root, e := readRoot(opt.TrustDir)
	if e != nil {
		return result, e
	}
	if !opt.Now.Before(root.Signed.Expires) {
		return result, errors.New("root expired; rotate offline")
	}
	snapshot, e := publisher.LoadSnapshot(opt.State)
	if e != nil {
		return result, e
	}
	release := strings.TrimRight(opt.ReleaseBase, "/") + "/"
	if b, e := os.ReadFile(filepath.Join(opt.State, ".distribution-version")); e == nil {
		var previous int64
		if _, e = fmt.Sscan(string(b), &previous); e != nil || opt.Version <= previous {
			return result, errors.New("publication version must advance")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return result, e
	}
	result = Index{Format: "romd-signed-catalog-1", Version: opt.Version, Snapshot: snapshot, Downloads: map[string]Asset{}}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	currentBytes, e := os.ReadFile(filepath.Join(opt.State, "current.json"))
	if e != nil {
		return result, e
	}
	var current publisher.Current
	if e = json.Unmarshal(currentBytes, &current); e != nil {
		return result, e
	}
	required := map[string]publisher.Reference{current.Snapshot.Path: current.Snapshot, snapshot.Feed.Path: snapshot.Feed}
	for _, catalog := range snapshot.Catalogs {
		if catalog.Artifact != nil {
			required[catalog.Artifact.Path] = *catalog.Artifact
		}
	}
	for _, event := range snapshot.Events {
		required[event.Artifact.Path] = event.Artifact
	}
	names := make([]string, 0, len(required))
	for name := range required {
		names = append(names, name)
	}
	sort.Strings(names)
	total := len(currentBytes)
	for _, name := range names {
		ref := required[name]
		if ref.Bytes < 0 || ref.Bytes > MaxState-total {
			return result, errors.New("publication state limit exceeded")
		}
		b, e := publisher.VerifiedRead(opt.State, ref)
		if e != nil {
			return result, e
		}
		total += len(b)
		if e = save(opt.Output, "assets/"+filepath.Base(name), b); e != nil {
			return result, e
		}
		w, e := zw.Create(name)
		if e != nil {
			return result, e
		}
		if _, e = w.Write(b); e != nil {
			return result, e
		}
		result.Downloads[name] = asset(release+filepath.Base(name), b)
	}
	b, e := os.ReadFile(filepath.Join(opt.State, "current.json"))
	if e != nil {
		return result, e
	}
	w, e := zw.Create("current.json")
	if e != nil {
		return result, e
	}
	if _, e = w.Write(b); e != nil {
		return result, e
	}
	if e = zw.Close(); e != nil {
		return result, e
	}
	if archive.Len() > MaxState {
		return result, errors.New("compressed state limit")
	}
	result.State = asset(release+"state.zip", archive.Bytes())
	if e = save(opt.Output, "assets/state.zip", archive.Bytes()); e != nil {
		return result, e
	}
	indexBytes, e := json.Marshal(result)
	if e != nil {
		return result, e
	}
	feed, e := publisher.VerifiedRead(opt.State, snapshot.Feed)
	if e != nil {
		return result, e
	}
	// The local publisher's immutable RSS references use this fixed placeholder.
	// Rewrite only that literal prefix to the verified release's flat asset paths.
	for path, a := range result.Downloads {
		feed = bytes.ReplaceAll(feed, []byte("https://catalogs.example.invalid/"+path), []byte(a.URL))
	}
	// The RSS channel itself links to the companion repository.
	repositoryURL := strings.Split(release, "/releases/download/")[0] + "/"
	feed = bytes.ReplaceAll(feed, []byte("https://catalogs.example.invalid/"), []byte(repositoryURL))
	targets := metadata.Targets(opt.Now.Add(30 * 24 * time.Hour))
	targets.Signed.Version = opt.Version
	site := filepath.Join(opt.Output, "site")
	for name, data := range map[string][]byte{"catalog.json": indexBytes, "feed.xml": feed} {
		target, e := metadata.TargetFile().FromBytes(name, data, "sha256")
		if e != nil {
			return result, e
		}
		targets.Signed.Targets[name] = target
		if e = save(site, "targets/"+publisher.Hash(data)+"."+name, data); e != nil {
			return result, e
		}
	}
	targetSigner, e := signer(opt.Keys, "targets")
	if e != nil {
		return result, e
	}
	if _, e = targets.Sign(targetSigner); e != nil {
		return result, e
	}
	if e = root.VerifyDelegate("targets", targets); e != nil {
		return result, e
	}
	tb, e := targets.ToBytes(false)
	if e != nil {
		return result, e
	}
	snap := metadata.Snapshot(opt.Now.Add(7 * 24 * time.Hour))
	snap.Signed.Version = opt.Version
	snap.Signed.Meta["targets.json"] = metaFile(opt.Version, tb)
	ss, e := signer(opt.Keys, "snapshot")
	if e != nil {
		return result, e
	}
	if _, e = snap.Sign(ss); e != nil {
		return result, e
	}
	if e = root.VerifyDelegate("snapshot", snap); e != nil {
		return result, e
	}
	sb, e := snap.ToBytes(false)
	if e != nil {
		return result, e
	}
	timestamp := metadata.Timestamp(opt.Now.Add(48 * time.Hour))
	timestamp.Signed.Version = opt.Version
	timestamp.Signed.Meta["snapshot.json"] = metaFile(opt.Version, sb)
	ts, e := signer(opt.Keys, "timestamp")
	if e != nil {
		return result, e
	}
	if _, e = timestamp.Sign(ts); e != nil {
		return result, e
	}
	if e = root.VerifyDelegate("timestamp", timestamp); e != nil {
		return result, e
	}
	tsb, e := timestamp.ToBytes(false)
	if e != nil {
		return result, e
	}
	for name, data := range map[string][]byte{fmt.Sprintf("metadata/%d.targets.json", opt.Version): tb, fmt.Sprintf("metadata/%d.snapshot.json", opt.Version): sb, "metadata/timestamp.json": tsb, "feed.xml": feed} {
		if e = save(site, name, data); e != nil {
			return result, e
		}
	}
	// Preserve the full root rotation chain for clients returning after downtime.
	roots, e := os.ReadDir(opt.TrustDir)
	if e != nil {
		return result, e
	}
	for _, r := range roots {
		if strings.HasSuffix(r.Name(), ".root.json") {
			b, e := os.ReadFile(filepath.Join(opt.TrustDir, r.Name()))
			if e != nil {
				return result, e
			}
			if e = save(site, "metadata/"+r.Name(), b); e != nil {
				return result, e
			}
		}
	}
	if e = save(site, "index.html", []byte("<!doctype html><meta charset=utf-8><title>ROMD DAT catalogs</title><h1>ROMD DAT catalog publisher</h1><p>Signed catalog updates. Consult the authenticated catalog index for available sources and acquisition health. RSS is a notification feed; clients verify signed metadata before applying updates.</p><p><a href=feed.xml>RSS feed</a></p>")); e != nil {
		return result, e
	}
	return result, nil
}

// RestoreState accepts a previously authenticated index and verified download.
// Extraction is bounded and never overwrites an existing publisher directory.
func RestoreState(index Index, raw []byte, destination string) error {
	if len(raw) != index.State.Bytes || publisher.Hash(raw) != index.State.SHA256 {
		return errors.New("state integrity failure")
	}
	if len(raw) > MaxState {
		return errors.New("state size limit")
	}
	if _, e := os.Stat(destination); !errors.Is(e, os.ErrNotExist) {
		return errors.New("restore destination must not exist")
	}
	z, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if e != nil {
		return e
	}
	tmp, e := os.MkdirTemp(filepath.Dir(destination), ".restore-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(tmp)
	total := 0
	seen := map[string]bool{}
	for _, f := range z.File {
		if seen[f.Name] || !f.Mode().IsRegular() || strings.Contains(f.Name, "\\") || strings.Contains(f.Name, "..") || strings.HasPrefix(f.Name, "/") || (f.Name != "current.json" && !strings.HasPrefix(f.Name, "objects/")) {
			return errors.New("unsafe state archive")
		}
		seen[f.Name] = true
		r, e := f.Open()
		if e != nil {
			return e
		}
		b, e := io.ReadAll(io.LimitReader(r, int64(MaxState-total)+1))
		r.Close()
		if e != nil {
			return e
		}
		total += len(b)
		if total > MaxState {
			return errors.New("expanded state limit")
		}
		if e = save(tmp, f.Name, b); e != nil {
			return e
		}
	}
	restored, e := publisher.LoadSnapshot(tmp)
	if e != nil {
		return e
	}
	a, _ := json.Marshal(restored)
	b, _ := json.Marshal(index.Snapshot)
	if !bytes.Equal(a, b) {
		return errors.New("state/index mismatch")
	}
	if e = save(tmp, ".distribution-version", []byte(fmt.Sprint(index.Version))); e != nil {
		return e
	}
	return os.Rename(tmp, destination)
}
