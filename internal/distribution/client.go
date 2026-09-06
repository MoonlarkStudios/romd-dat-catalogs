package distribution

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/JackSkylark/romd-dat-catalogs/internal/publisher"
	"github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/updater"
)

// RefreshIndex starts from an explicitly trusted root, never a root fetched on
// first use. Keep cache across refreshes for rollback detection. Call serially.
func RefreshIndex(root []byte, site, cache string, client *http.Client) (Index, error) {
	var index Index
	cfg, e := config.New(site+"/metadata/", root)
	if e != nil {
		return index, e
	}
	cfg.RemoteTargetsURL = site + "/targets/"
	cfg.LocalMetadataDir = filepath.Join(cache, "metadata")
	cfg.LocalTargetsDir = filepath.Join(cache, "targets")
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if e = cfg.SetDefaultFetcherHTTPClient(client); e != nil {
		return index, e
	}
	if e = cfg.SetDefaultFetcherRetry(time.Millisecond, 1); e != nil {
		return index, e
	}
	up, e := updater.New(cfg)
	if e != nil {
		return index, e
	}
	if e = up.Refresh(); e != nil {
		return index, e
	}
	target, e := up.GetTargetInfo("catalog.json")
	if e != nil {
		return index, e
	}
	if target.Length > publisher.MaxMetadata {
		return index, errors.New("catalog index size limit")
	}
	name, _, e := up.DownloadTarget(target, "", "")
	if e != nil {
		return index, e
	}
	b, e := os.ReadFile(name)
	if e != nil {
		return index, e
	}
	if e = json.Unmarshal(b, &index); e != nil {
		return index, e
	}
	if index.Format != "romd-signed-catalog-1" || index.Version < 1 {
		return index, errors.New("unsupported signed catalog")
	}
	return index, nil
}

var assetURL = regexp.MustCompile(`^https://github.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/releases/download/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func FetchAsset(a Asset, client *http.Client) ([]byte, error) {
	if !assetURL.MatchString(a.URL) {
		return nil, errors.New("expected public GitHub Release asset URL")
	}
	if a.Bytes < 1 || a.Bytes > MaxState {
		return nil, errors.New("asset size limit")
	}
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	response, e := client.Get(a.URL)
	if e != nil {
		return nil, e
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("asset download failed")
	}
	b, e := io.ReadAll(io.LimitReader(response.Body, int64(a.Bytes)+1))
	if e != nil {
		return nil, e
	}
	if len(b) != a.Bytes || publisher.Hash(b) != a.SHA256 {
		return nil, errors.New("asset integrity failure")
	}
	return b, nil
}
