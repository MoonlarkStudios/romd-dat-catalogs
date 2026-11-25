package distribution

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

const (
	mirrorExpiryWarning = 12 * time.Hour
	mirrorFailureLimit  = 2
)

type MirrorPublication struct {
	Version int64     `json:"version"`
	Expires time.Time `json:"expires"`
}

type MirrorReport struct {
	CheckedAt           time.Time          `json:"checkedAt"`
	CatalogID           string             `json:"catalogId"`
	LastVerified        *MirrorPublication `json:"lastVerifiedPublication,omitempty"`
	SecondsUntilExpiry  *int64             `json:"secondsUntilExpiry,omitempty"`
	UpstreamHealth      string             `json:"upstreamHealth,omitempty"`
	LastUpstreamSuccess *string            `json:"lastUpstreamSuccess,omitempty"`
	Verified            bool               `json:"verified"`
	ConsecutiveFailures int                `json:"consecutiveFailures"`
	Failure             string             `json:"failure,omitempty"`
	Alert               string             `json:"alert,omitempty"`
}

type mirrorState struct {
	Identity            string             `json:"identity"`
	LastVerified        *MirrorPublication `json:"lastVerifiedPublication,omitempty"`
	ConsecutiveFailures int                `json:"consecutiveFailures"`
}

// MonitorMirror checks the same signed index and DAT bytes as the normal reader.
// The caller must hold the reader lock for a dedicated, persistent cache.
func MonitorMirror(root []byte, site, cache, catalogID, name string, metadataClient, assetClient *http.Client) (MirrorReport, error) {
	identity := publisher.Hash([]byte(strings.Join([]string{publisher.Hash(root), site, catalogID, name}, "\x00")))
	path := filepath.Join(cache, "monitor.json")
	state, err := readMirrorState(path, identity)
	if err != nil {
		return MirrorReport{}, err
	}

	report := probeMirror(root, site, cache, catalogID, name, metadataClient, assetClient)
	if report.LastVerified != nil {
		state.LastVerified = report.LastVerified
	}
	if report.Verified {
		state.ConsecutiveFailures = 0
	} else if state.ConsecutiveFailures < mirrorFailureLimit {
		state.ConsecutiveFailures++
	}
	report.CheckedAt = time.Now().UTC()
	report.LastVerified = state.LastVerified
	report.ConsecutiveFailures = state.ConsecutiveFailures
	report.evaluateAlert()
	return report, writeMirrorState(path, state)
}

func probeMirror(root []byte, site, cache, catalogID, name string, metadataClient, assetClient *http.Client) MirrorReport {
	report := MirrorReport{CatalogID: catalogID}
	up, err := refreshUpdater(root, site, cache, metadataClient)
	if err != nil {
		report.Failure = "directory_verification_failed"
		if errors.Is(err, &metadata.ErrExpiredMetadata{}) {
			report.Failure = "metadata_expired"
		}
		return report
	}
	index, err := readIndex(up)
	if err != nil || index.Definitions == nil {
		report.Failure = "directory_verification_failed"
		return report
	}
	timestamp := up.GetTrustedMetadataSet().Timestamp.Signed
	if timestamp.Version != index.Version {
		report.Failure = "publication_version_mismatch"
		return report
	}
	report.LastVerified = &MirrorPublication{Version: index.Version, Expires: timestamp.Expires}
	if catalog, ok := index.Snapshot.Catalogs[catalogID]; ok {
		report.UpstreamHealth = catalog.Health
		report.LastUpstreamSuccess = catalog.LastSuccessfulCheck
	}
	if _, _, err = ReadCandidate(index, catalogID, name, assetClient); err != nil {
		report.Failure = "candidate_verification_failed"
		return report
	}
	report.Verified = true
	return report
}

func (r *MirrorReport) evaluateAlert() {
	var remaining time.Duration
	if r.LastVerified != nil {
		remaining = r.LastVerified.Expires.Sub(r.CheckedAt)
		seconds := int64(remaining.Seconds())
		r.SecondsUntilExpiry = &seconds
	}
	switch {
	case r.Failure == "metadata_expired" || (r.LastVerified != nil && remaining <= 0):
		r.Alert = "metadata_expired"
	case r.LastVerified != nil && remaining <= mirrorExpiryWarning:
		r.Alert = "metadata_expiring"
	case r.ConsecutiveFailures >= mirrorFailureLimit:
		r.Alert = "repeated_verification_failure"
	}
}

func readMirrorState(path, identity string) (mirrorState, error) {
	state := mirrorState{Identity: identity}
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(body, &state); err != nil {
		return state, err
	}
	if state.Identity != identity || state.ConsecutiveFailures < 0 || state.ConsecutiveFailures > mirrorFailureLimit ||
		(state.LastVerified != nil && (state.LastVerified.Version < 1 || state.LastVerified.Expires.IsZero())) {
		return state, errors.New("invalid or mismatched mirror monitor state; use a dedicated cache")
	}
	return state, nil
}

func writeMirrorState(path string, state mirrorState) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".monitor-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	err = json.NewEncoder(file).Encode(state)
	if err = errors.Join(err, file.Close()); err != nil {
		return fmt.Errorf("save mirror monitor state: %w", err)
	}
	return os.Rename(file.Name(), path)
}
