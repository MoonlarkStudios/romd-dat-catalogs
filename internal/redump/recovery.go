package redump

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

// CheckpointStore is authoritative remote custody. CompareAndSwap must reject
// stale versions; an empty version may create an absent checkpoint only.
// An ambiguous write error must stop the caller, never permit acquisition.
type CheckpointStore interface {
	Load(context.Context) ([]byte, string, error)
	CompareAndSwap(context.Context, string, []byte) (string, error)
}

func NewRemoteScheduler(lockDirectory string, store CheckpointStore) (*Scheduler, error) {
	if store == nil {
		return nil, errors.New("remote checkpoint store required")
	}
	s := NewScheduler(lockDirectory)
	s.store = store
	return s, nil
}
func (s *Scheduler) load(ctx context.Context) (sourceState, string, error) {
	if s.store == nil {
		state, e := readState(s.root)
		return state, "", e
	}
	raw, version, e := s.store.Load(ctx)
	if e != nil {
		return sourceState{}, "", e
	}
	if version == "" {
		return sourceState{}, "", errors.New("checkpoint version missing")
	}
	state, e := decodeState(raw)
	return state, version, e
}
func (s *Scheduler) commit(ctx context.Context, state sourceState, version *string) error {
	if s.store == nil {
		return saveState(s.root, state)
	}
	raw, e := json.Marshal(state)
	if e != nil {
		return e
	}
	if len(raw) > maxStateBytes {
		return errors.New("source state size limit")
	}
	next, e := s.store.CompareAndSwap(ctx, *version, raw)
	if e != nil {
		return e
	}
	if next == "" {
		return errors.New("checkpoint receipt missing")
	}
	*version = next
	return nil
}

type RecoveryRecord struct {
	At                 time.Time `json:"at"`
	InterruptedAttempt time.Time `json:"interruptedAttempt"`
	NotBefore          time.Time `json:"notBefore"`
	Disposition        string    `json:"disposition"`
}
type Review struct {
	Digest        string    `json:"digest"`
	InFlight      bool      `json:"inFlight"`
	LastAttempt   time.Time `json:"lastAttempt"`
	NextAttempt   time.Time `json:"nextAttempt"`
	RecoveryCount uint64    `json:"recoveryCount"`
}
type RecoveryOptions struct {
	ExpectedDigest                                            string
	NotBefore                                                 time.Time
	ConfirmedStopped, ConfirmedRetryWindow, DiscardCandidates bool
}

func review(state sourceState) Review {
	b, _ := json.Marshal(state)
	return Review{publisher.Hash(b), state.InFlight, state.LastAttempt, state.NextAttempt, state.RecoveryCount}
}
func (s *Scheduler) Inspect(ctx context.Context) (Review, error) {
	if s.store != nil {
		if e := os.MkdirAll(s.root, 0700); e != nil {
			return Review{}, e
		}
	}
	unlock, e := lockState(s.root)
	if e != nil {
		return Review{}, e
	}
	defer unlock()
	state, _, e := s.load(ctx)
	if e != nil {
		return Review{}, e
	}
	return review(state), nil
}

// Recover resolves an explicitly reviewed interrupted run without acquiring or
// publishing anything. Candidate discard is a disposition, not filesystem deletion.
// The operator must stop the old runner and establish the upstream retry window.
func (s *Scheduler) Recover(ctx context.Context, opt RecoveryOptions) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !opt.ConfirmedStopped || !opt.ConfirmedRetryWindow || !opt.DiscardCandidates {
		return errors.New("stopped runner, retry-window review, and candidate disposition required")
	}
	if s.store != nil {
		if e := os.MkdirAll(s.root, 0700); e != nil {
			return e
		}
	}
	unlock, e := lockState(s.root)
	if e != nil {
		return e
	}
	defer unlock()
	state, version, e := s.load(ctx)
	if e != nil {
		return e
	}
	if !state.InFlight {
		return errors.New("source is not interrupted")
	}
	if review(state).Digest != opt.ExpectedDigest {
		return errors.New("recovery review is stale")
	}
	now := s.now().UTC()
	if now.IsZero() || now.Before(state.LastAttempt) || opt.NotBefore.IsZero() || opt.NotBefore.Before(now) || opt.NotBefore.Before(state.NextAttempt) {
		return errors.New("recovery cannot shorten retry deadline or regress time")
	}
	if state.RecoveryCount == ^uint64(0) {
		return errors.New("recovery counter exhausted")
	}
	state.RecoveryCount++
	state.Recovery = &RecoveryRecord{At: now, InterruptedAttempt: state.LastAttempt, NotBefore: opt.NotBefore.UTC(), Disposition: "discard-unpublished-candidates"}
	state.NextAttempt = opt.NotBefore.UTC()
	state.InFlight = false
	return s.commit(ctx, state, &version)
}
