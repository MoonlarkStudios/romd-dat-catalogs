package redump

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func interrupted(t *testing.T) (*Scheduler, sourceState) {
	t.Helper()
	s := NewScheduler(filepath.Join(t.TempDir(), "source"))
	now := time.Now().UTC()
	s.now = func() time.Time { return now }
	if e := s.Initialize([]Catalog{catalog("psx")}); e != nil {
		t.Fatal(e)
	}
	state, e := readState(s.root)
	if e != nil {
		t.Fatal(e)
	}
	state.LastAttempt = now.Add(-time.Hour)
	state.NextAttempt = now.Add(48 * time.Hour)
	state.InFlight = true
	if e = saveState(s.root, state); e != nil {
		t.Fatal(e)
	}
	return s, state
}
func TestReviewedRecoveryPreservesBindingsAndDeadline(t *testing.T) {
	s, before := interrupted(t)
	rev, e := s.Inspect(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	opt := RecoveryOptions{rev.Digest, before.NextAttempt.Add(time.Hour), true, true, true}
	if e = s.Recover(context.Background(), opt); e != nil {
		t.Fatal(e)
	}
	after, e := readState(s.root)
	if e != nil {
		t.Fatal(e)
	}
	if after.InFlight || after.RecoveryCount != 1 || after.Recovery == nil || after.NextAttempt.Before(before.NextAttempt) || !reflect.DeepEqual(before.Registry, after.Registry) || before.Origin != after.Origin || before.LastAttempt != after.LastAttempt {
		t.Fatalf("bad recovery: %+v", after)
	}
	if e = s.Recover(context.Background(), opt); e == nil {
		t.Fatal("replayed recovery")
	}
	if _, e = runScheduler(t, s, catalog("psx")); !errors.Is(e, ErrNotDue) {
		t.Fatal(e)
	}
}
func TestRecoveryRejectsUnsafeOrStaleReview(t *testing.T) {
	for _, change := range []func(*RecoveryOptions){func(o *RecoveryOptions) { o.ExpectedDigest = "stale" }, func(o *RecoveryOptions) { o.NotBefore = time.Now() }, func(o *RecoveryOptions) { o.ConfirmedStopped = false }, func(o *RecoveryOptions) { o.ConfirmedRetryWindow = false }, func(o *RecoveryOptions) { o.DiscardCandidates = false }} {
		s, state := interrupted(t)
		rev, e := s.Inspect(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		opt := RecoveryOptions{rev.Digest, state.NextAttempt, true, true, true}
		change(&opt)
		if e = s.Recover(context.Background(), opt); e == nil {
			t.Fatal("unsafe recovery accepted")
		}
		after, e := s.Inspect(context.Background())
		if e != nil || after.Digest != rev.Digest {
			t.Fatal("rejected recovery changed state")
		}
	}
}
func TestLegacyStateUpgradesWithoutReset(t *testing.T) {
	s, state := interrupted(t)
	state.Format = "romd-redump-state-1"
	if e := saveState(s.root, state); e != nil {
		t.Fatal(e)
	}
	decoded, e := readState(s.root)
	if e != nil || decoded.Format != stateFormat || decoded.LastAttempt != state.LastAttempt || !decoded.InFlight {
		t.Fatalf("migration %+v %v", decoded, e)
	}
}

type memoryCheckpoint struct {
	mu                          sync.Mutex
	raw                         []byte
	version, calls, ambiguousAt int
	barrier                     chan struct{}
	loads                       int
}

func (m *memoryCheckpoint) Load(context.Context) ([]byte, string, error) {
	m.mu.Lock()
	raw := append([]byte(nil), m.raw...)
	version := strconv.Itoa(m.version)
	m.loads++
	b := m.barrier
	if b != nil && m.loads == 2 {
		close(b)
		m.barrier = nil
	}
	m.mu.Unlock()
	if b != nil {
		<-b
	}
	if len(raw) == 0 {
		return nil, "", errors.New("missing checkpoint")
	}
	return raw, version, nil
}
func (m *memoryCheckpoint) CompareAndSwap(_ context.Context, v string, raw []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := strconv.Itoa(m.version)
	if m.version == 0 {
		want = ""
	}
	if v != want {
		return "", errors.New("checkpoint conflict")
	}
	m.raw = append([]byte(nil), raw...)
	m.version++
	m.calls++
	if m.calls == m.ambiguousAt {
		return "", errors.New("ambiguous write")
	}
	return strconv.Itoa(m.version), nil
}
func remote(t *testing.T, m *memoryCheckpoint, a *Adapter) *Scheduler {
	t.Helper()
	s, e := NewRemoteScheduler(filepath.Join(t.TempDir(), "locks"), m)
	if e != nil {
		t.Fatal(e)
	}
	s.adapter = a
	return s
}
func TestRemoteAdmissionBeforeRequestAndAmbiguousWrite(t *testing.T) {
	var requests atomic.Int32
	m := &memoryCheckpoint{ambiguousAt: 2}
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) { requests.Add(1) })
	s := remote(t, m, a)
	c := catalog("psx")
	if e := s.Initialize([]Catalog{c}); e != nil {
		t.Fatal(e)
	}
	if _, e := runScheduler(t, s, c); e == nil {
		t.Fatal("ambiguous admission accepted")
	}
	restarted := remote(t, m, newAdapter(a.origin, a.client.Transport))
	if _, e := runScheduler(t, restarted, c); !errors.Is(e, ErrInterrupted) {
		t.Fatalf("lost remote admission: %v", e)
	}
	if requests.Load() != 0 {
		t.Fatal("upstream before remote receipt")
	}
}
func TestCompetingRemoteRunnersOnlyOneAcquires(t *testing.T) {
	raw := fixture(t)
	var requests atomic.Int32
	m := &memoryCheckpoint{}
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		m.mu.Lock()
		state, e := decodeState(m.raw)
		m.mu.Unlock()
		if e != nil || !state.InFlight {
			t.Error("missing durable admission")
		}
		w.Write(raw)
	})
	first := remote(t, m, a)
	second := remote(t, m, newAdapter(a.origin, a.client.Transport))
	c := catalog("psx")
	if e := first.Initialize([]Catalog{c}); e != nil {
		t.Fatal(e)
	}
	m.barrier = make(chan struct{})
	stages := []string{filepath.Join(t.TempDir(), "one"), filepath.Join(t.TempDir(), "two")}
	done := make(chan error, 2)
	for i, s := range []*Scheduler{first, second} {
		go func(s *Scheduler, stage string) { _, e := s.Run(context.Background(), []Catalog{c}, stage); done <- e }(s, stages[i])
	}
	e1, e2 := <-done, <-done
	if (e1 == nil) == (e2 == nil) || requests.Load() != 1 {
		t.Fatalf("admission %v %v requests %d", e1, e2, requests.Load())
	}
}
func TestRemoteRetryDeadlineSurvivesAmbiguousCheckpointAndRecovery(t *testing.T) {
	m := &memoryCheckpoint{ambiguousAt: 3}
	retry := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", retry.Format(http.TimeFormat))
		w.WriteHeader(429)
	})
	s := remote(t, m, a)
	c := catalog("psx")
	if e := s.Initialize([]Catalog{c}); e != nil {
		t.Fatal(e)
	}
	if _, e := runScheduler(t, s, c); e == nil {
		t.Fatal("ambiguous deadline accepted")
	}
	restarted := remote(t, m, newAdapter(a.origin, a.client.Transport))
	rev, e := restarted.Inspect(context.Background())
	if e != nil || !rev.InFlight || !rev.NextAttempt.Equal(retry) {
		t.Fatalf("retry lost %+v %v", rev, e)
	}
	if e = restarted.Recover(context.Background(), RecoveryOptions{rev.Digest, retry, true, true, true}); e != nil {
		t.Fatal(e)
	}
	if _, e = runScheduler(t, restarted, c); !errors.Is(e, ErrNotDue) {
		t.Fatal(e)
	}
}
func TestRemoteMissingCheckpointNeverBootstraps(t *testing.T) {
	s := remote(t, &memoryCheckpoint{}, New())
	if _, e := runScheduler(t, s, catalog("psx")); e == nil {
		t.Fatal("missing remote accepted")
	}
}
func TestRecoveryRecordValidation(t *testing.T) {
	_, state := interrupted(t)
	state.RecoveryCount = 1
	b, _ := json.Marshal(state)
	if _, e := decodeState(b); e == nil {
		t.Fatal("missing audit accepted")
	}
}
