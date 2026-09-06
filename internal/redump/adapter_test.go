package redump

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("../../fixtures/example.dat")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func catalog(system string) Catalog {
	return Catalog{ID: "redump/" + system + "/discs", System: system, ExpectedName: "ROMD Synthetic Console", Platform: "synthetic-console", Representation: "discs", PolicyVersion: "1", MinGames: 1, MinROMs: 1}
}
func adapter(t *testing.T, h http.HandlerFunc) *Adapter {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return newAdapter(s.URL, s.Client().Transport)
}
func acquire(t *testing.T, a *Adapter, cs ...Catalog) []Result {
	t.Helper()
	r, e := a.Acquire(context.Background(), cs, filepath.Join(t.TempDir(), "stage"))
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func packed(t *testing.T, b []byte, comment string) []byte {
	t.Helper()
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	if e := z.SetComment(comment); e != nil {
		t.Fatal(e)
	}
	w, e := z.Create("catalog.dat")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = w.Write(b); e != nil {
		t.Fatal(e)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}

func TestAcquisitionPublicationRepackagingAndFailureRetention(t *testing.T) {
	raw := fixture(t)
	var mode atomic.Int32
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/datfile/psx/" {
			t.Error("unexpected path")
		}
		if mode.Load() == 3 {
			fmt.Fprint(w, "<html>temporarily unavailable</html>")
			return
		}
		if mode.Load() == 0 {
			w.Write(raw)
			return
		}
		w.Write(packed(t, raw, strconv.Itoa(int(mode.Load()))))
	})
	root := filepath.Join(t.TempDir(), "published")
	var previous publisher.Snapshot
	for i := 0; i < 4; i++ {
		mode.Store(int32(i))
		results := acquire(t, a, catalog("psx"))
		if i < 3 && (results[0].Code != "" || results[0].SHA256 != publisher.Hash(raw)) {
			t.Fatalf("candidate: %+v", results[0])
		}
		s, e := publisher.Publish(root, []publisher.Attempt{results[0].Attempt}, "https://catalogs.example.invalid/", publisher.Options{Now: time.Date(2026, 9, 6, 0, 0, i, 0, time.UTC)})
		if e != nil {
			t.Fatal(e)
		}
		if len(s.Events) != 1 {
			t.Fatalf("duplicate events: %d", len(s.Events))
		}
		if i > 0 && *s.Catalogs[catalog("psx").ID].Artifact != *previous.Catalogs[catalog("psx").ID].Artifact {
			t.Fatal("lost prior artifact")
		}
		if i == 3 && s.Catalogs[catalog("psx").ID].Health != "failed" {
			t.Fatal("failure not reported")
		}
		previous = s
	}
}
func TestInvalidCandidatesAndResponseBounds(t *testing.T) {
	raw := fixture(t)
	cases := []struct {
		name, code string
		serve      http.HandlerFunc
		floor      int
	}{
		{"wrong identity", "invalid_document", func(w http.ResponseWriter, r *http.Request) {
			w.Write(bytes.ReplaceAll(raw, []byte("ROMD Synthetic Console"), []byte("Other Console")))
		}, 1},
		{"empty", "invalid_document", func(w http.ResponseWriter, r *http.Request) {}, 1},
		{"truncated XML", "invalid_document", func(w http.ResponseWriter, r *http.Request) { w.Write(raw[:len(raw)/2]) }, 1},
		{"count collapse", "count_below_floor", func(w http.ResponseWriter, r *http.Request) { w.Write(raw) }, 2},
		{"missing", "http_status", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }, 1},
		{"declared size", "input_limit", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(publisher.MaxInput+1))
		}, 1},
		{"streamed size", "input_limit", func(w http.ResponseWriter, r *http.Request) {
			w.(http.Flusher).Flush()
			w.Write(bytes.Repeat([]byte("x"), publisher.MaxInput+1))
		}, 1},
		{"truncated body", "incomplete_response", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "1000")
			fmt.Fprint(w, "short")
		}, 1},
		{"encoding", "unexpected_encoding", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Encoding", "gzip"); w.Write(raw) }, 1},
		{"expanded archive", "invalid_document", func(w http.ResponseWriter, r *http.Request) {
			w.Write(packed(t, bytes.Repeat([]byte("x"), publisher.MaxDocument+1), ""))
		}, 1},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			a := adapter(t, tt.serve)
			c := catalog("psx")
			c.MinGames = tt.floor
			r := acquire(t, a, c)[0]
			if r.Code != tt.code || r.Attempt.Path != nil || r.Attempt.Failure == nil {
				t.Fatalf("result %+v", r)
			}
		})
	}
}
func TestRedirectNeverContactsTarget(t *testing.T) {
	var called atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called.Add(1) }))
	defer target.Close()
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) })
	r := acquire(t, a, catalog("psx"))[0]
	if r.Code != "http_status" || called.Load() != 0 {
		t.Fatal("followed redirect")
	}
}
func TestPerCatalogFailureIsolation(t *testing.T) {
	raw := fixture(t)
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "bad") {
			w.WriteHeader(404)
		} else {
			w.Write(raw)
		}
	})
	r := acquire(t, a, catalog("bad"), catalog("good"))
	if r[0].Code != "http_status" || r[1].Code != "" {
		t.Fatalf("results %+v", r)
	}
}
func TestProviderBackoffStopsBatch(t *testing.T) {
	for _, status := range []int{429, 503} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var calls atomic.Int32
			a := adapter(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", "3600")
				w.WriteHeader(status)
			})
			before := time.Now()
			r := acquire(t, a, catalog("one"), catalog("two"))
			if calls.Load() != 1 || r[1].Code != "provider_backoff" || r[0].RetryAt.Before(before.Add(time.Hour)) || !r[0].RetryAt.Equal(r[1].RetryAt) {
				t.Fatalf("backoff %+v", r)
			}
		})
	}
}
func TestRetryAfterParsing(t *testing.T) {
	now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	for _, s := range []string{"", "invalid", "-1", now.Add(-time.Hour).Format(http.TimeFormat)} {
		if !retryAt(s, now).Equal(now.Add(time.Minute)) {
			t.Fatal(s)
		}
	}
	future := now.Add(2 * time.Hour)
	if !retryAt(future.Format(http.TimeFormat), now).Equal(future) {
		t.Fatal("HTTP date")
	}
	if !retryAt("99999999999999999999999999", now).After(now.AddDate(100, 0, 0)) {
		t.Fatal("overflow shortened cooldown")
	}
}
func TestCancellationAndStagingOwnership(t *testing.T) {
	var calls atomic.Int32
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stage := filepath.Join(t.TempDir(), "stage")
	r, e := a.Acquire(ctx, []Catalog{catalog("psx")}, stage)
	if e == nil || len(r) != 0 || calls.Load() != 0 {
		t.Fatalf("cancel %v %+v", e, r)
	}
	if e = os.Mkdir(stage, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e = a.Acquire(context.Background(), []Catalog{catalog("psx")}, stage); e == nil {
		t.Fatal("reused staging")
	}
}
func TestInvalidRegistryMakesNoRequests(t *testing.T) {
	var calls atomic.Int32
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	bad := catalog("../escape")
	missing := catalog("psx")
	missing.PolicyVersion = ""
	for _, cs := range [][]Catalog{nil, {bad}, {missing}, {catalog("psx"), catalog("psx")}, make([]Catalog, MaxCatalogs+1)} {
		if _, e := a.Acquire(context.Background(), cs, filepath.Join(t.TempDir(), "stage")); e == nil {
			t.Fatal("accepted invalid registry")
		}
	}
	for _, origin := range []string{"https://redump.org", "ftp://redump.org", "http://", "http://user:secret@redump.org", "http://redump.org/path", "http://redump.org?", "http://redump.org?q=x", "http://redump.org#fragment"} {
		a.origin = origin
		if _, e := a.Acquire(context.Background(), []Catalog{catalog("psx")}, filepath.Join(t.TempDir(), "stage")); e == nil {
			t.Fatal("accepted invalid origin")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid config accessed network")
	}
}

func TestCooldownSurvivesNextCall(t *testing.T) {
	var calls atomic.Int32
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(429)
	})
	first := acquire(t, a, catalog("one"))[0]
	second := acquire(t, a, catalog("two"))[0]
	if calls.Load() != 1 || second.Code != "provider_backoff" || !first.RetryAt.Equal(second.RetryAt) {
		t.Fatal("cooldown was not retained")
	}
}
func TestCancellationDuringBodyAndAdmission(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	a := adapter(t, func(w http.ResponseWriter, r *http.Request) { w.(http.Flusher).Flush(); <-release })
	defer close(release)
	transport := a.client.Transport
	a.client.Transport = requestTransport(func(r *http.Request) (*http.Response, error) {
		response, err := transport.RoundTrip(r)
		if err == nil {
			response.Body = &observedBody{ReadCloser: response.Body, entered: entered}
		}
		return response, err
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan []Result, 1)
	errCh := make(chan error, 1)
	stage := filepath.Join(t.TempDir(), "first")
	go func() { r, e := a.Acquire(ctx, []Catalog{catalog("one")}, stage); done <- r; errCh <- e }()
	<-entered
	blocked, cancelBlocked := context.WithCancel(context.Background())
	cancelBlocked()
	if _, e := a.Acquire(blocked, []Catalog{catalog("two")}, filepath.Join(t.TempDir(), "second")); e == nil {
		t.Fatal("cancelled admission accepted")
	}
	cancel()
	r := <-done
	if e := <-errCh; e != nil {
		t.Fatal(e)
	}
	if len(r) != 1 || r[0].Code != "incomplete_response" || r[0].Attempt.Path != nil {
		t.Fatalf("cancelled body: %+v", r)
	}
}

// A server-side Flush does not prove the client has received headers. Observe
// the body read itself so cancellation exercises the intended failure boundary.
type observedBody struct {
	io.ReadCloser
	entered chan struct{}
	once    sync.Once
}

func (b *observedBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered) })
	return b.ReadCloser.Read(p)
}

type requestTransport func(*http.Request) (*http.Response, error)

func (f requestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProductionAdapterRequestsHTTPAndRecordsSource(t *testing.T) {
	raw := fixture(t)
	a := New()
	calls := 0
	a.client.Transport = requestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodGet || r.URL.String() != "http://redump.org/datfile/psx/" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Accept-Encoding") != "identity" || r.Header.Get("User-Agent") == "" {
			t.Fatal("unexpected acquisition headers")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(raw)), ContentLength: int64(len(raw)), Request: r}, nil
	})
	result := acquire(t, a, catalog("psx"))[0]
	if calls != 1 || result.Code != "" || result.SHA256 != publisher.Hash(raw) || result.Attempt.SourceURL != "http://redump.org/datfile/psx/" {
		t.Fatalf("HTTP acquisition: %+v; calls=%d", result, calls)
	}
}
