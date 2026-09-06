package redump

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"
)

var (
	ErrNotDue          = errors.New("source check not yet due")
	ErrRegistryChanged = errors.New("source registry changed; reviewed migration required")
	ErrInterrupted     = errors.New("incomplete source run; recovery review required")
	ErrBusy            = errors.New("source state is locked by another run")
)

const stateFormat = "romd-redump-state-2"
const pollInterval = 24 * time.Hour
const maxStateBytes = 1 << 20

type sourceState struct {
	Format        string          `json:"format"`
	Origin        string          `json:"origin"`
	Registry      []Catalog       `json:"registry"`
	InFlight      bool            `json:"inFlight"`
	LastAttempt   time.Time       `json:"lastAttempt"`
	NextAttempt   time.Time       `json:"nextAttempt"`
	RecoveryCount uint64          `json:"recoveryCount,omitempty"`
	Recovery      *RecoveryRecord `json:"recovery,omitempty"`
}

// Scheduler persists admission and provider cooldown on a local POSIX filesystem.
// It does not publish results or reset state automatically. The state directory
// must survive process restarts; ephemeral CI runners need separate state custody.
type Scheduler struct {
	root    string
	adapter *Adapter
	now     func() time.Time
	store   CheckpointStore
}

func NewScheduler(root string) *Scheduler {
	return &Scheduler{root: root, adapter: New(), now: time.Now}
}
func canonical(catalogs []Catalog) []Catalog {
	c := append([]Catalog(nil), catalogs...)
	sort.Slice(c, func(i, j int) bool { return c[i].ID < c[j].ID })
	return c
}

// Initialize is an explicit one-time operation; an existing directory is refused.
func (s *Scheduler) Initialize(catalogs []Catalog) error {
	if e := validateCatalogs(catalogs); e != nil {
		return e
	}
	if e := os.Mkdir(s.root, 0700); e != nil {
		return e
	}
	unlock, e := lockState(s.root)
	if e != nil {
		return e
	}
	defer unlock()
	version := ""
	return s.commit(context.Background(), sourceState{Format: stateFormat, Origin: s.adapter.origin, Registry: canonical(catalogs)}, &version)
}

// Run records in-flight admission before any network request. Runs without a
// committed completion remain paused rather than guessing a safe retry time.
// RetryAt is synced after each result, before returning candidates to the caller.
func (s *Scheduler) Run(ctx context.Context, catalogs []Catalog, stage string) ([]Result, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if e := validateCatalogs(catalogs); e != nil {
		return nil, e
	}
	if s.store != nil {
		if e := os.MkdirAll(s.root, 0700); e != nil {
			return nil, e
		}
	}
	unlock, e := lockState(s.root)
	if e != nil {
		return nil, e
	}
	defer unlock()
	state, version, e := s.load(ctx)
	if e != nil {
		return nil, e
	}
	if state.Origin != s.adapter.origin || !reflect.DeepEqual(state.Registry, canonical(catalogs)) {
		return nil, ErrRegistryChanged
	}
	if state.InFlight {
		return nil, ErrInterrupted
	}
	now := s.now().UTC()
	if now.IsZero() || now.Before(state.LastAttempt) {
		return nil, errors.New("source clock regressed")
	}
	if now.Before(state.NextAttempt) {
		return nil, ErrNotDue
	}
	if _, e = os.Lstat(stage); !errors.Is(e, os.ErrNotExist) {
		return nil, errors.New("staging directory must be new")
	}
	state.InFlight = true
	state.LastAttempt = now
	state.NextAttempt = now.Add(pollInterval)
	if e = s.commit(ctx, state, &version); e != nil {
		return nil, e
	}
	results, e := s.adapter.acquire(ctx, canonical(catalogs), stage, func(r Result) error {
		if r.RetryAt.After(state.NextAttempt) {
			state.NextAttempt = r.RetryAt
			return s.commit(ctx, state, &version)
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	state.InFlight = false
	if e = s.commit(ctx, state, &version); e != nil {
		return nil, e
	}
	return results, nil
}
func readState(root string) (sourceState, error) {
	var state sourceState
	f, e := os.Open(filepath.Join(root, "state.json"))
	if e != nil {
		return state, e
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, maxStateBytes+1))
	if e != nil {
		return state, e
	}
	return decodeState(raw)
}
func decodeState(raw []byte) (sourceState, error) {
	var state sourceState
	if len(raw) > maxStateBytes {
		return state, errors.New("source state size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&state); e != nil {
		return state, e
	}
	var extra any
	if e := decoder.Decode(&extra); e != io.EOF {
		return state, errors.New("trailing source state data")
	}
	if state.Format == "romd-redump-state-1" && state.Recovery == nil && state.RecoveryCount == 0 {
		state.Format = stateFormat
	}
	if (state.RecoveryCount == 0) != (state.Recovery == nil) {
		return state, errors.New("invalid recovery history")
	}
	if r := state.Recovery; r != nil {
		if r.Disposition != "discard-unpublished-candidates" || r.At.Before(r.InterruptedAttempt) || r.At.After(r.NotBefore) || r.InterruptedAttempt.IsZero() || r.InterruptedAttempt.After(state.LastAttempt) || r.NotBefore.Before(r.InterruptedAttempt.Add(pollInterval)) || r.NotBefore.After(state.NextAttempt) {
			return state, errors.New("invalid recovery record")
		}
	}
	if state.Format != stateFormat || state.Origin == "" || validateCatalogs(state.Registry) != nil || !reflect.DeepEqual(state.Registry, canonical(state.Registry)) {
		return state, errors.New("invalid source state")
	}
	if state.LastAttempt.IsZero() != state.NextAttempt.IsZero() || (!state.LastAttempt.IsZero() && state.NextAttempt.Before(state.LastAttempt.Add(pollInterval))) || (state.InFlight && state.LastAttempt.IsZero()) {
		return state, errors.New("invalid source schedule")
	}
	return state, nil
}
func saveState(root string, state sourceState) error {
	b, e := json.Marshal(state)
	if e != nil {
		return e
	}
	if len(b) > maxStateBytes {
		return errors.New("source state size limit")
	}
	f, e := os.CreateTemp(root, ".state-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, e = f.Write(b); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Rename(f.Name(), filepath.Join(root, "state.json")); e != nil {
		return e
	}
	dir, e := os.Open(root)
	if e != nil {
		return e
	}
	defer dir.Close()
	return dir.Sync()
}
