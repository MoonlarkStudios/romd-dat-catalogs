package redump

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func scheduler(t *testing.T, a *Adapter, now time.Time) *Scheduler {
	t.Helper()
	return &Scheduler{root: filepath.Join(t.TempDir(), "source"), adapter: a, now: func() time.Time { return now }}
}
func runScheduler(t *testing.T, s *Scheduler, cs ...Catalog) ([]Result, error) {
	t.Helper()
	return s.Run(context.Background(), cs, filepath.Join(t.TempDir(), "stage"))
}
func TestDurableCooldownAcrossSchedulerRestart(t *testing.T) {
	raw := fixture(t)
	var calls atomic.Int32
	now := time.Now().UTC()
	retry := now.Add(48 * time.Hour).Truncate(time.Second)
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", retry.Format(http.TimeFormat))
			w.WriteHeader(429)
		} else {
			w.Write(raw)
		}
	})
	s := scheduler(t, a, now)
	c := catalog("psx")
	if e := s.Initialize([]Catalog{c}); e != nil {
		t.Fatal(e)
	}
	result, e := runScheduler(t, s, c)
	if e != nil || result[0].Code != "rate_limited" {
		t.Fatalf("first run: %v %+v", e, result)
	}
	restarted := &Scheduler{root: s.root, adapter: newAdapter(a.origin, a.client.Transport), now: func() time.Time { return now.Add(25 * time.Hour) }}
	if _, e = runScheduler(t, restarted, c); !errors.Is(e, ErrNotDue) {
		t.Fatalf("lost cooldown: %v", e)
	}
	if calls.Load() != 1 {
		t.Fatal("accessed upstream during cooldown")
	}
	restarted.now = func() time.Time { return retry.Add(time.Second) }
	if _, e = runScheduler(t, restarted, c); e != nil {
		t.Fatal(e)
	}
	state, e := readState(s.root)
	if e != nil || state.InFlight || calls.Load() != 2 {
		t.Fatalf("completion: %+v %v", state, e)
	}
}
func TestDurableRegistryRejectsChangesBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	original := catalog("psx")
	changes := []func(*Catalog){func(c *Catalog) { c.Platform = "other" }, func(c *Catalog) { c.PolicyVersion = "2" }, func(c *Catalog) { c.MinGames++ }, func(c *Catalog) { c.System = "other" }, func(c *Catalog) { c.ID = "redump/other/discs" }, func(c *Catalog) { c.Representation = "bios" }, func(c *Catalog) { c.ExpectedName = "other" }}
	for _, change := range changes {
		s := scheduler(t, a, time.Now())
		if e := s.Initialize([]Catalog{original}); e != nil {
			t.Fatal(e)
		}
		changed := original
		change(&changed)
		if _, e := runScheduler(t, s, changed); !errors.Is(e, ErrRegistryChanged) {
			t.Fatal(e)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("changed registry reached network")
	}
}
func TestRegistryOrderAndDailyAdmission(t *testing.T) {
	raw := fixture(t)
	var calls atomic.Int32
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.Write(raw) })
	now := time.Now()
	s := scheduler(t, a, now)
	one, two := catalog("one"), catalog("two")
	if e := s.Initialize([]Catalog{two, one}); e != nil {
		t.Fatal(e)
	}
	if _, e := runScheduler(t, s, one, two); e != nil {
		t.Fatal(e)
	}
	if _, e := runScheduler(t, s, two, one); !errors.Is(e, ErrNotDue) {
		t.Fatal(e)
	}
	state, e := readState(s.root)
	if e != nil || !state.NextAttempt.Equal(now.UTC().Add(24*time.Hour)) || calls.Load() != 2 {
		t.Fatalf("state: %+v %v", state, e)
	}
}
func TestMissingCorruptStateAndRepeatedInitializationFailClosed(t *testing.T) {
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected network request") })
	c := catalog("psx")
	for _, raw := range []string{"", `{`, `{"format":"unknown"}`, `{"format":"romd-redump-state-1","registry":[]}`} {
		s := scheduler(t, a, time.Now())
		if e := s.Initialize([]Catalog{c}); e != nil {
			t.Fatal(e)
		}
		if e := s.Initialize([]Catalog{c}); e == nil {
			t.Fatal("reinitialized state")
		}
		p := filepath.Join(s.root, "state.json")
		if raw == "" {
			if e := os.Remove(p); e != nil {
				t.Fatal(e)
			}
		} else if e := os.WriteFile(p, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := runScheduler(t, s, c); e == nil {
			t.Fatal("accepted missing/corrupt state")
		}
	}
}
func TestAdmissionSurvivesCrashProcess(t *testing.T) {
	s := NewScheduler(filepath.Join(t.TempDir(), "source"))
	c := catalog("psx")
	if e := s.Initialize([]Catalog{c}); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestSchedulerProcessHelper$")
	cmd.Env = append(os.Environ(), "ROMD_SCHEDULER_HELPER=crash", "ROMD_SCHEDULER_STATE="+s.root, "ROMD_SCHEDULER_STAGE="+filepath.Join(t.TempDir(), "stage"))
	e := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(e, &exit) || exit.ExitCode() != 91 {
		t.Fatalf("expected injected crash: %v", e)
	}
	state, e := readState(s.root)
	if e != nil || !state.InFlight {
		t.Fatalf("lost admission: %+v %v", state, e)
	}
	if _, e := runScheduler(t, s, c); !errors.Is(e, ErrInterrupted) {
		t.Fatalf("crash silently retried: %v", e)
	}
}
func TestSeparateProcessLockAndStableInode(t *testing.T) {
	s := NewScheduler(filepath.Join(t.TempDir(), "source"))
	c := catalog("psx")
	if e := s.Initialize([]Catalog{c}); e != nil {
		t.Fatal(e)
	}
	before, e := os.Stat(filepath.Join(s.root, ".source.lock"))
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestSchedulerProcessHelper$")
	cmd.Env = append(os.Environ(), "ROMD_SCHEDULER_HELPER=lock", "ROMD_SCHEDULER_STATE="+s.root)
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	stdin, e := cmd.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer cmd.Process.Kill()
	if line, e := bufio.NewReader(stdout).ReadString('\n'); e != nil || line != "locked\n" {
		t.Fatalf("helper: %q %v", line, e)
	}
	if _, e = runScheduler(t, s, c); !errors.Is(e, ErrBusy) {
		t.Fatalf("concurrent process admitted: %v", e)
	}
	stdin.Close()
	if e = cmd.Wait(); e != nil {
		t.Fatal(e)
	}
	after, e := os.Stat(filepath.Join(s.root, ".source.lock"))
	if e != nil || !os.SameFile(before, after) {
		t.Fatal("lock inode replaced")
	}
	unlock, e := lockState(s.root)
	if e != nil {
		t.Fatal(e)
	}
	unlock()
}

type crashTransport struct{}

func (crashTransport) RoundTrip(*http.Request) (*http.Response, error) {
	os.Exit(91)
	return nil, errors.New("unreachable")
}
func TestSchedulerProcessHelper(t *testing.T) {
	mode := os.Getenv("ROMD_SCHEDULER_HELPER")
	if mode == "" {
		return
	}
	root := os.Getenv("ROMD_SCHEDULER_STATE")
	if mode == "lock" {
		unlock, e := lockState(root)
		if e != nil {
			t.Fatal(e)
		}
		defer unlock()
		fmt.Println("locked")
		bufio.NewReader(os.Stdin).ReadByte()
		return
	}
	s := NewScheduler(root)
	s.adapter = New()
	s.adapter.client.Transport = crashTransport{}
	if _, e := s.Run(context.Background(), []Catalog{catalog("psx")}, os.Getenv("ROMD_SCHEDULER_STAGE")); e != nil {
		t.Fatal(e)
	}
	t.Fatal("expected process termination")
}

func TestScheduleCorruptionAndClockRegressionFailClosed(t *testing.T) {
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") })
	now := time.Now().UTC()
	s := scheduler(t, a, now)
	c := catalog("psx")
	if e := s.Initialize([]Catalog{c}); e != nil {
		t.Fatal(e)
	}
	state, e := readState(s.root)
	if e != nil {
		t.Fatal(e)
	}
	state.LastAttempt = now.Add(time.Hour)
	state.NextAttempt = state.LastAttempt.Add(24 * time.Hour)
	if e = saveState(s.root, state); e != nil {
		t.Fatal(e)
	}
	if _, e = runScheduler(t, s, c); e == nil || errors.Is(e, ErrNotDue) {
		t.Fatalf("clock regression not rejected: %v", e)
	}
	state.NextAttempt = state.LastAttempt
	if e = saveState(s.root, state); e != nil {
		t.Fatal(e)
	}
	if _, e = runScheduler(t, s, c); e == nil {
		t.Fatal("accepted corrupted schedule")
	}
}
func TestInvalidStateFileMakesNoRequest(t *testing.T) {
	var calls atomic.Int32
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	s := scheduler(t, a, time.Now())
	c := catalog("psx")
	if e := s.Initialize([]Catalog{c}); e != nil {
		t.Fatal(e)
	}
	// A corrupt/missing state is never replaced with a new empty scheduling record.
	if e := os.Remove(filepath.Join(s.root, "state.json")); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(filepath.Join(s.root, "state.json"), 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := runScheduler(t, s, c); e == nil {
		t.Fatal("invalid state accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("network before valid admission")
	}
}
