package checkpoint

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func client(t *testing.T, h http.HandlerFunc) *GitHub {
	t.Helper()
	s := httptest.NewTLSServer(h)
	t.Cleanup(s.Close)
	g, e := NewGitHub("example/catalogs", "test-token")
	if e != nil {
		t.Fatal(e)
	}
	g.endpoint = s.URL + "/state"
	g.client.Transport = s.Client().Transport
	return g
}
func file(raw []byte) map[string]any {
	return map[string]any{"type": "file", "encoding": "base64", "content": base64.StdEncoding.EncodeToString(raw), "size": len(raw), "sha": blobSHA(raw)}
}
func TestGitHubCreateLoadAndConditionalUpdate(t *testing.T) {
	var mu sync.Mutex
	var raw []byte
	g := client(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("X-GitHub-Api-Version") == "" {
			t.Error("missing authentication/API version")
		}
		if r.Method == "GET" {
			if r.URL.Query().Get("ref") != Branch {
				t.Error("wrong branch")
			}
			if raw == nil {
				w.WriteHeader(404)
				return
			}
			json.NewEncoder(w).Encode(file(raw))
			return
		}
		var p map[string]string
		if e := json.NewDecoder(r.Body).Decode(&p); e != nil {
			t.Error(e)
		}
		if p["branch"] != Branch {
			t.Error("write targeted wrong branch")
		}
		expected := ""
		if raw != nil {
			expected = blobSHA(raw)
		}
		if p["sha"] != expected {
			w.WriteHeader(409)
			return
		}
		var e error
		raw, e = base64.StdEncoding.DecodeString(p["content"])
		if e != nil {
			t.Error(e)
		}
		json.NewEncoder(w).Encode(map[string]any{"content": map[string]string{"sha": blobSHA(raw)}})
	})
	ctx := context.Background()
	if _, _, e := g.Load(ctx); e == nil {
		t.Fatal("missing state accepted")
	}
	first := []byte(`{"state":1}`)
	v, e := g.CompareAndSwap(ctx, "", first)
	if e != nil {
		t.Fatal(e)
	}
	got, loaded, e := g.Load(ctx)
	if e != nil || string(got) != string(first) || loaded != v {
		t.Fatalf("readback %v", e)
	}
	next, e := g.CompareAndSwap(ctx, v, []byte(`{"state":2}`))
	if e != nil || next == v {
		t.Fatal("update failed", e)
	}
	if _, e = g.CompareAndSwap(ctx, v, first); e != ErrConflict {
		t.Fatal("stale revision accepted", e)
	}
	if _, e = g.CompareAndSwap(ctx, "", first); e != ErrConflict {
		t.Fatal("reinitialized remote state", e)
	}
}
func TestGitHubRejectsBadResponses(t *testing.T) {
	for _, name := range []string{"hash", "encoding", "size", "html", "missing", "denied", "oversize"} {
		t.Run(name, func(t *testing.T) {
			g := client(t, func(w http.ResponseWriter, r *http.Request) {
				f := file([]byte("state"))
				switch name {
				case "hash":
					f["sha"] = strings.Repeat("0", 40)
				case "encoding":
					f["content"] = "bad!"
				case "size":
					f["size"] = MaxBytes + 1
				case "html":
					w.Write([]byte("<html>error</html>"))
					return
				case "missing":
					w.WriteHeader(404)
					return
				case "denied":
					w.WriteHeader(403)
					return
				case "oversize":
					w.Write([]byte(strings.Repeat("x", 2*MaxBytes+1)))
					return
				}
				json.NewEncoder(w).Encode(f)
			})
			if _, _, e := g.Load(context.Background()); e == nil {
				t.Fatal("bad checkpoint accepted")
			}
		})
	}
}
func TestGitHubRedirectDoesNotForwardToken(t *testing.T) {
	var calls atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer other.Close()
	g := client(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, 302) })
	if _, _, e := g.Load(context.Background()); e == nil || calls.Load() != 0 {
		t.Fatal("followed checkpoint redirect")
	}
}
func TestGitHubWriteReceiptAndInputValidation(t *testing.T) {
	var calls atomic.Int32
	g := client(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(`{"content":{"sha":"wrong"}}`))
	})
	if _, e := g.CompareAndSwap(context.Background(), "", []byte("state")); e == nil {
		t.Fatal("bad receipt accepted")
	}
	for _, v := range []string{"bad", strings.Repeat("a", 39)} {
		if _, e := g.CompareAndSwap(context.Background(), v, []byte("x")); e == nil {
			t.Fatal("bad version accepted")
		}
	}
	if _, e := g.CompareAndSwap(context.Background(), "", []byte(strings.Repeat("x", MaxBytes+1))); e == nil {
		t.Fatal("oversize write accepted")
	}
	if calls.Load() != 1 {
		t.Fatal("invalid input reached network")
	}
	if _, e := NewGitHub("../escape", "token"); e == nil {
		t.Fatal("bad repo")
	}
	if _, e := NewGitHub("a/b", ""); e == nil {
		t.Fatal("missing token")
	}
}
