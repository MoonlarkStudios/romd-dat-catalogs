package distribution

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
)

type fixture struct {
	dir, keys, state, trust string
	now                     time.Time
	online                  Keys
	root                    []byte
	location                atomic.Value
	server                  *httptest.Server
}

func makeFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{dir: t.TempDir(), now: time.Now().UTC()}
	f.keys = filepath.Join(f.dir, "keys")
	f.state = filepath.Join(f.dir, "state")
	if e := Initialize(f.keys, f.now, nil, nil); e != nil {
		t.Fatal(e)
	}
	f.trust = filepath.Join(f.keys, "public")
	f.root = read(t, filepath.Join(f.trust, "1.root.json"))
	var e error
	f.online, e = ReadKeys(filepath.Join(f.keys, "online.json"))
	if e != nil {
		t.Fatal(e)
	}
	p := "../../fixtures/example.dat"
	_, e = publisher.Publish(f.state, []publisher.Attempt{{CatalogID: "synthetic/console/standard", ExpectedName: "ROMD Synthetic Console", SourceURL: "https://example.invalid/dat", Path: &p}}, "https://catalogs.example.invalid/", publisher.Options{Now: f.now})
	if e != nil {
		t.Fatal(e)
	}
	f.location.Store(f.dir)
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(http.Dir(f.location.Load().(string))).ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)
	return f
}
func read(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func (f *fixture) stage(t *testing.T, version int64) (Index, string) {
	t.Helper()
	out := filepath.Join(f.dir, fmt.Sprintf("stage-%d", version))
	index, e := Stage(StageOptions{State: f.state, TrustDir: f.trust, Output: out, ReleaseBase: fmt.Sprintf("https://github.com/MoonlarkStudios/romd-dat-catalogs/releases/download/test-%d", version), Keys: f.online, Version: version, Now: f.now})
	if e != nil {
		t.Fatal(e)
	}
	return index, out
}
func (f *fixture) refresh(cache string) (Index, error) {
	return RefreshIndex(f.root, f.server.URL+"/site", cache, f.server.Client())
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func (f *fixture) assetClient() *http.Client {
	return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		u, _ := url.Parse(f.server.URL + "/assets/" + filepath.Base(r.URL.Path))
		request := r.Clone(r.Context())
		request.URL = u
		return http.DefaultTransport.RoundTrip(request)
	})}
}

func TestSignedPublicationAndRestore(t *testing.T) {
	f := makeFixture(t)
	expected, stage := f.stage(t, 1)
	f.location.Store(stage)
	index, e := f.refresh(filepath.Join(f.dir, "cache"))
	if e != nil {
		t.Fatal(e)
	}
	if index.State.SHA256 != expected.State.SHA256 {
		t.Fatal("wrong authenticated index")
	}
	data, e := FetchAsset(index.State, f.assetClient())
	if e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(f.dir, "restored")
	if e = RestoreState(index, data, dest); e != nil {
		t.Fatal(e)
	}
	restored, e := publisher.LoadSnapshot(dest)
	if e != nil || restored.Sequence != index.Snapshot.Sequence {
		t.Fatalf("restore: %v", e)
	}
	for _, asset := range index.Downloads {
		if _, e = FetchAsset(asset, f.assetClient()); e != nil {
			t.Fatal(e)
		}
	}
	feed := read(t, filepath.Join(stage, "site/feed.xml"))
	if bytes.Contains(feed, []byte("catalogs.example.invalid")) {
		t.Fatal("placeholder in RSS")
	}
	if !bytes.Contains(feed, []byte("releases/download/test-1/")) {
		t.Fatal("release enclosure missing")
	}
}
func TestUntrustedRootRejected(t *testing.T) {
	f := makeFixture(t)
	_, stage := f.stage(t, 1)
	f.location.Store(stage)
	other := filepath.Join(f.dir, "other")
	if e := Initialize(other, f.now, nil, nil); e != nil {
		t.Fatal(e)
	}
	if _, e := RefreshIndex(read(t, filepath.Join(other, "public/1.root.json")), f.server.URL+"/site", filepath.Join(f.dir, "cache"), f.server.Client()); e == nil {
		t.Fatal("untrusted root accepted")
	}
}
func TestTamperedTargetRejected(t *testing.T) {
	f := makeFixture(t)
	_, stage := f.stage(t, 1)
	f.location.Store(stage)
	files, e := filepath.Glob(filepath.Join(stage, "site/targets/*.catalog.json"))
	if e != nil || len(files) != 1 {
		t.Fatal("target missing")
	}
	if e = os.WriteFile(files[0], []byte("tampered"), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = f.refresh(filepath.Join(f.dir, "cache")); e == nil {
		t.Fatal("tampered index accepted")
	}
}
func TestExpiredTimestampAndRoot(t *testing.T) {
	f := makeFixture(t)
	_, stage := f.stage(t, 1)
	trusted, e := trustedmetadata.New(f.root)
	if e != nil {
		t.Fatal(e)
	}
	trusted.RefTime = f.now.Add(49 * time.Hour)
	if _, e = trusted.UpdateTimestamp(read(t, filepath.Join(stage, "site/metadata/timestamp.json"))); e == nil {
		t.Fatal("expired timestamp accepted")
	}
	_, e = Stage(StageOptions{State: f.state, TrustDir: f.trust, Output: filepath.Join(f.dir, "expired"), ReleaseBase: "https://github.com/o/r/releases/download/a", Keys: f.online, Version: 2, Now: f.now.AddDate(2, 0, 0)})
	if e == nil {
		t.Fatal("expired root publication")
	}
}
func TestRollbackRejectedWithPersistentCache(t *testing.T) {
	f := makeFixture(t)
	_, one := f.stage(t, 1)
	_, two := f.stage(t, 2)
	cache := filepath.Join(f.dir, "cache")
	f.location.Store(two)
	if _, e := f.refresh(cache); e != nil {
		t.Fatal(e)
	}
	f.location.Store(one)
	if _, e := f.refresh(cache); e == nil {
		t.Fatal("rollback accepted")
	}
}
func TestMixAndMatchRejected(t *testing.T) {
	f := makeFixture(t)
	_, one := f.stage(t, 1)
	_, two := f.stage(t, 2)
	b := read(t, filepath.Join(two, "site/metadata/2.snapshot.json"))
	if e := os.WriteFile(filepath.Join(one, "site/metadata/1.snapshot.json"), b, 0644); e != nil {
		t.Fatal(e)
	}
	f.location.Store(one)
	if _, e := f.refresh(filepath.Join(f.dir, "cache")); e == nil {
		t.Fatal("mixed snapshot accepted")
	}
}
func TestRotationWithOldBootstrap(t *testing.T) {
	f := makeFixture(t)
	_, first := f.stage(t, 1)
	f.location.Store(first)
	cache := filepath.Join(f.dir, "cache")
	if _, e := f.refresh(cache); e != nil {
		t.Fatal(e)
	}
	old, e := ReadKeys(filepath.Join(f.keys, "offline-root.json"))
	if e != nil {
		t.Fatal(e)
	}
	next := filepath.Join(f.dir, "next-keys")
	if e = Initialize(next, f.now, f.root, old); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(f.trust, "2.root.json"), read(t, filepath.Join(next, "public/2.root.json")), 0644); e != nil {
		t.Fatal(e)
	}
	f.online, e = ReadKeys(filepath.Join(next, "online.json"))
	if e != nil {
		t.Fatal(e)
	}
	_, second := f.stage(t, 2)
	f.location.Store(second)
	if _, e = f.refresh(cache); e != nil {
		t.Fatal(e)
	}
	if _, e = f.refresh(filepath.Join(f.dir, "new-client")); e != nil {
		t.Fatal(e)
	}
}
func TestRotationRequiresOldKey(t *testing.T) {
	f := makeFixture(t)
	wrong := filepath.Join(f.dir, "wrong")
	if e := Initialize(wrong, f.now, nil, nil); e != nil {
		t.Fatal(e)
	}
	keys, e := ReadKeys(filepath.Join(wrong, "offline-root.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = Initialize(filepath.Join(f.dir, "rotation"), f.now, f.root, keys); e == nil {
		t.Fatal("unauthorized rotation")
	}
}
func TestOldOnlineKeysRejectedAfterRotation(t *testing.T) {
	f := makeFixture(t)
	old, e := ReadKeys(filepath.Join(f.keys, "offline-root.json"))
	if e != nil {
		t.Fatal(e)
	}
	next := filepath.Join(f.dir, "next")
	if e = Initialize(next, f.now, f.root, old); e != nil {
		t.Fatal(e)
	}
	f.trust = filepath.Join(next, "public")
	_, e = Stage(StageOptions{State: f.state, TrustDir: f.trust, Output: filepath.Join(f.dir, "staged"), ReleaseBase: "https://github.com/o/r/releases/download/a", Keys: f.online, Version: 1, Now: f.now})
	if e == nil {
		t.Fatal("old signer accepted")
	}
	if _, e = os.Stat(filepath.Join(f.dir, "staged/site/metadata/timestamp.json")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("published timestamp despite signing failure")
	}
}
func TestFailedStagingLeavesLivePublication(t *testing.T) {
	f := makeFixture(t)
	_, live := f.stage(t, 1)
	f.location.Store(live)
	before := read(t, filepath.Join(live, "site/metadata/timestamp.json"))
	bad := Keys{}
	_, e := Stage(StageOptions{State: f.state, TrustDir: f.trust, Output: filepath.Join(f.dir, "failed"), ReleaseBase: "https://github.com/o/r/releases/download/a", Keys: bad, Version: 2, Now: f.now})
	if e == nil {
		t.Fatal("missing key accepted")
	}
	if !bytes.Equal(before, read(t, filepath.Join(live, "site/metadata/timestamp.json"))) {
		t.Fatal("live pointer modified")
	}
	if _, e = f.refresh(filepath.Join(f.dir, "cache")); e != nil {
		t.Fatal(e)
	}
}
func TestAssetsMustExistAndMatchBeforePromotion(t *testing.T) {
	f := makeFixture(t)
	index, stage := f.stage(t, 1)
	f.location.Store(stage)
	if e := os.Remove(filepath.Join(stage, "assets/state.zip")); e != nil {
		t.Fatal(e)
	}
	if _, e := FetchAsset(index.State, f.assetClient()); e == nil {
		t.Fatal("missing asset accepted")
	}
	if e := os.WriteFile(filepath.Join(stage, "assets/state.zip"), []byte("tampered"), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := FetchAsset(index.State, f.assetClient()); e == nil {
		t.Fatal("tampered asset accepted")
	}
}
func TestRestoreNeverOverwritesAndChecksIndex(t *testing.T) {
	f := makeFixture(t)
	index, stage := f.stage(t, 1)
	raw := read(t, filepath.Join(stage, "assets/state.zip"))
	if e := RestoreState(index, raw, f.state); e == nil {
		t.Fatal("overwrote existing state")
	}
	index.Snapshot.Sequence++
	if e := RestoreState(index, raw, filepath.Join(f.dir, "invalid")); e == nil {
		t.Fatal("state/index mismatch")
	}
}
func TestRestoredPublicationVersionMustAdvance(t *testing.T) {
	f := makeFixture(t)
	index, stage := f.stage(t, 2)
	restored := filepath.Join(f.dir, "restored")
	if e := RestoreState(index, read(t, filepath.Join(stage, "assets/state.zip")), restored); e != nil {
		t.Fatal(e)
	}
	_, e := Stage(StageOptions{State: restored, TrustDir: f.trust, Output: filepath.Join(f.dir, "rollback"), ReleaseBase: "https://github.com/o/r/releases/download/a", Keys: f.online, Version: 1, Now: f.now})
	if e == nil {
		t.Fatal("publication version regressed")
	}
}
func TestNoPrivateKeysInPublication(t *testing.T) {
	f := makeFixture(t)
	_, stage := f.stage(t, 1)
	e := filepath.WalkDir(stage, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		for _, key := range f.online {
			encoded, _ := json.Marshal(key)
			if bytes.Contains(b, bytes.Trim(encoded, "\"")) {
				return errors.New("private key in publication")
			}
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if info, e := os.Stat(filepath.Join(f.keys, "online.json")); e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("key file permissions")
	}
}
func TestDuplicateInitializationRejected(t *testing.T) {
	f := makeFixture(t)
	if e := Initialize(f.keys, f.now, nil, nil); e == nil {
		t.Fatal("overwrote keys")
	}
}
func TestRootPrivateKeyRejectedByStage(t *testing.T) {
	f := makeFixture(t)
	keys, e := ReadKeys(filepath.Join(f.keys, "offline-root.json"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = Stage(StageOptions{State: f.state, TrustDir: f.trust, Output: filepath.Join(f.dir, "bad"), ReleaseBase: "https://github.com/o/r/releases/download/a", Keys: keys, Version: 1, Now: f.now})
	if e == nil || !strings.Contains(e.Error(), "offline root") {
		t.Fatal("root exposed to publisher")
	}
}
