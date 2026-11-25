package distribution

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
)

// PublicationCheckpoint identifies a previously verified deployment. Operators
// record these values independently of recovery, using the successful deployment
// run and its authenticated catalog index.
type PublicationCheckpoint struct {
	Version                       int64
	Repository, Commit, IndexHash string
	VerifiedAt                    time.Time
}

// RecoverPublisher restores a historical publishing input. It does not refresh a
// consumer cache or make the old publication current. The next stage must sign
// a higher version using the current clock and the existing online role keys.
func RecoverPublisher(root []byte, site, destination string, checkpoint PublicationCheckpoint, client *http.Client) error {
	origin, err := url.Parse(site)
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" {
		return errors.New("recovery requires the existing HTTPS publishing site")
	}
	now := time.Now().UTC()
	digest, err := hex.DecodeString(checkpoint.IndexHash)
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != checkpoint.IndexHash ||
		checkpoint.Version < 1 || !gitRepository.MatchString(checkpoint.Repository) ||
		!gitCommit.MatchString(checkpoint.Commit) || checkpoint.VerifiedAt.IsZero() || checkpoint.VerifiedAt.After(now) {
		return errors.New("recovery requires a verified publication time, version, index SHA-256 and immutable data commit")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	index, err := readCheckpoint(root, strings.TrimRight(site, "/"), checkpoint, now, client)
	if err != nil {
		return err
	}
	return RestorePublication(index, destination, client)
}

func readCheckpoint(root []byte, site string, checkpoint PublicationCheckpoint, now time.Time, client *http.Client) (Index, error) {
	var index Index
	trusted, err := trustedmetadata.New(root)
	if err != nil {
		return index, err
	}
	trusted.RefTime = checkpoint.VerifiedAt
	if !trusted.Root.Signed.ConsistentSnapshot {
		return index, errors.New("checkpoint requires consistent snapshots")
	}
	for rotations := 0; ; rotations++ {
		if rotations == 32 {
			return index, errors.New("checkpoint root rotation limit")
		}
		path := fmt.Sprintf("metadata/%d.root.json", trusted.Root.Signed.Version+1)
		data, err := readCheckpointFile(client, site, path, true)
		if err != nil {
			return index, err
		}
		if data == nil {
			break
		}
		if _, err = trusted.UpdateRoot(data); err != nil {
			return index, err
		}
	}
	if trusted.Root.Signed.IsExpired(now) {
		return index, errors.New("checkpoint root is expired at the current time")
	}
	timestampData, err := readCheckpointFile(client, site, "metadata/timestamp.json", false)
	if err != nil {
		return index, err
	}
	timestamp, err := trusted.UpdateTimestamp(timestampData)
	if err != nil {
		return index, err
	}
	version := checkpoint.Version
	snapshotInfo := timestamp.Signed.Meta["snapshot.json"]
	if timestamp.Signed.Version != version || !matchesCheckpointVersion(snapshotInfo, version) {
		return index, errors.New("published timestamp and snapshot do not match the verified checkpoint version")
	}
	snapshotData, err := readCheckpointFile(client, site, fmt.Sprintf("metadata/%d.snapshot.json", version), false)
	if err != nil {
		return index, err
	}
	snapshot, err := trusted.UpdateSnapshot(snapshotData, false)
	if err != nil {
		return index, err
	}
	if !matchesCheckpointVersion(snapshot.Signed.Meta["targets.json"], version) {
		return index, errors.New("snapshot does not bind the checkpoint targets version")
	}
	targetsData, err := readCheckpointFile(client, site, fmt.Sprintf("metadata/%d.targets.json", version), false)
	if err != nil {
		return index, err
	}
	targets, err := trusted.UpdateTargets(targetsData)
	if err != nil {
		return index, err
	}
	target := targets.Signed.Targets["catalog.json"]
	if target == nil || hex.EncodeToString(target.Hashes["sha256"]) != checkpoint.IndexHash {
		return index, errors.New("signed index hash does not match the verified checkpoint")
	}
	raw, err := readCheckpointFile(client, site, "targets/"+checkpoint.IndexHash+".catalog.json", false)
	if err != nil {
		return index, err
	}
	if err = target.VerifyLengthHashes(raw); err != nil {
		return index, err
	}
	if err = json.Unmarshal(raw, &index); err != nil {
		return index, err
	}
	if index.Format != GitFormat || index.Version != version || index.Definitions == nil || len(index.Downloads) == 0 {
		return index, errors.New("checkpoint requires the matching Git publication and signed definitions")
	}
	prefix := "https://raw.githubusercontent.com/" + checkpoint.Repository + "/" + checkpoint.Commit + "/"
	for _, asset := range index.Downloads {
		if !gitAssetURL.MatchString(asset.URL) || !strings.HasPrefix(asset.URL, prefix) {
			return index, errors.New("checkpoint asset does not point to the verified data commit")
		}
	}
	return index, nil
}

func matchesCheckpointVersion(info *metadata.MetaFiles, version int64) bool {
	return info != nil && info.Version == version && info.Length > 0 && len(info.Hashes["sha256"]) == 32
}

func readCheckpointFile(client *http.Client, site, path string, optional bool) ([]byte, error) {
	response, err := client.Get(site + "/" + path)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if optional && response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checkpoint %s returned HTTP %d", path, response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, publisher.MaxMetadata+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > publisher.MaxMetadata {
		return nil, fmt.Errorf("checkpoint %s metadata size limit", path)
	}
	return data, nil
}
