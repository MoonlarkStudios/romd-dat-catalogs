package nointro

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
	"strings"
	"testing"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/definitions"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

func catalog() definitions.Catalog {
	return definitions.Catalog{SystemID: "snes", Provider: "no-intro", ProviderSystemID: "49", Representation: "standard", ExpectedName: "Nintendo - Super Nintendo Entertainment System", Validation: definitions.Validation{MinimumGames: 2, MinimumROMs: 2}}
}
func document() []byte {
	return []byte(`<?xml version="1.0"?><datafile><header><id>49</id><name>Nintendo - Super Nintendo Entertainment System</name><version>synthetic-1</version><homepage>No-Intro</homepage></header><game name="Fixture (Japan) (Proto)"><rom name="a.sfc" size="1" crc="01234567" md5="0123456789abcdef0123456789abcdef" sha1="0123456789abcdef0123456789abcdef01234567"/></game><game name="Fixture (World) (Aftermarket) (Unl)"><rom name="unknown.sfc" status="nodump"/></game></datafile>`)
}
func selection() string {
	s := `<form name="main_form" method="post" action=""><select name="system_selection"><option value="49" selected="selected">Nintendo - Super Nintendo Entertainment System</option></select>`
	for name, values := range completeSelection() {
		s += fmt.Sprintf(`<input type="radio" name="%s" value="%s">`, name, values[0])
	}
	return s + `<input type="submit" name="download_dat_2026-09-05" value="Prepare"></form>`
}

const manager = `<form action="" method="post"><input type="submit" name="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" value="Download!!" style="display:none;"></form><form action="" method="post"><input type="submit" name="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" value="Download!!"></form>`

func zipped(t *testing.T, b []byte, comment string) []byte {
	t.Helper()
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	if err := z.SetComment(comment); err != nil {
		t.Fatal(err)
	}
	w, err := z.Create("complete.dat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write(b); err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func TestAcquisitionLifecycle(t *testing.T) {
	payload := zipped(t, document(), "first")
	step := 0
	status := 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		if status != 200 {
			w.Header().Set("Retry-After", "7200")
			w.WriteHeader(status)
			return
		}
		if r.Header.Get("User-Agent") == "" || r.Header.Get("Accept-Encoding") != "identity" {
			t.Error("missing request policy")
		}
		if r.URL.Path != "/index.php" || r.URL.Query().Get("s") != "49" {
			t.Error("wrong system route")
		}
		switch (step - 1) % 4 {
		case 0:
			if r.Method != "GET" {
				t.Error("wrong method")
			}
			http.SetCookie(w, &http.Cookie{Name: "anonymous", Value: "fixture"})
			io.WriteString(w, selection())
		case 1:
			if _, err := r.Cookie("anonymous"); err != nil {
				t.Error("lost session")
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			for k, v := range completeSelection() {
				if r.PostForm.Get(k) != v[0] {
					t.Errorf("missing inclusion %s", k)
				}
			}
			if len(r.PostForm) != len(completeSelection())+1 {
				t.Error("unexpected posted controls")
			}
			w.Header().Set("Location", "index.php?page=manager&s=49&download=7")
			w.WriteHeader(302)
		case 2:
			io.WriteString(w, manager)
		case 3:
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.PostForm.Get("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb") != "Download!!" || len(r.PostForm) != 1 {
				t.Error("wrong visible control")
			}
			w.Write(payload)
		}
	}))
	defer server.Close()
	adapter := newAdapter(server.URL, server.Client().Transport, 0)
	state := filepath.Join(t.TempDir(), "state")
	acquire := func() publisher.Snapshot {
		t.Helper()
		result, err := adapter.Acquire(context.Background(), "no-intro/snes/standard", catalog(), filepath.Join(t.TempDir(), "input"))
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := publisher.Publish(state, []publisher.Attempt{result.Attempt}, "https://example.invalid/", publisher.Options{})
		if err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	first := acquire()
	old := first.Catalogs["no-intro/snes/standard"]
	if old.Artifact == nil || old.Artifact.SHA256 != publisher.Hash(document()) {
		t.Fatal("document not preserved", old)
	}
	payload = zipped(t, document(), "different packaging")
	same := acquire()
	if *same.Catalogs["no-intro/snes/standard"].Artifact != *old.Artifact || same.Events[0].ID != first.Events[0].ID {
		t.Fatal("repack published new content")
	}
	payload = bytes.Replace(document(), []byte("synthetic-1"), []byte("synthetic-2"), 1)
	changed := acquire()
	current := changed.Catalogs["no-intro/snes/standard"]
	if current.Artifact.SHA256 == old.Artifact.SHA256 || changed.Events[0].ID == first.Events[0].ID {
		t.Fatal("document change lost")
	}
	payload = []byte("<html>upstream error</html>")
	failed := acquire()
	if c := failed.Catalogs["no-intro/snes/standard"]; c.Health != "failed" || *c.Artifact != *current.Artifact {
		t.Fatal("invalid document lost working artifact")
	}
	status = 429
	limited := acquire()
	requests := step
	if limited.Catalogs["no-intro/snes/standard"].RetryAt == nil {
		t.Fatal("missing retry guidance")
	}
	acquire()
	if step != requests {
		t.Fatal("retried during cooldown")
	}
}
func TestFormsFailClosed(t *testing.T) {
	for name, form := range map[string]string{
		"missing aftermarket": strings.ReplaceAll(selection(), `<input type="radio" name="lifespan_2" value="0">`, ""),
		"new default":         strings.Replace(selection(), "</form>", `<input type="checkbox" name="new_filter" value="1"></form>`, 1),
		"wrong system":        strings.ReplaceAll(selection(), `value="49"`, `value="24"`),
		"wrong name":          strings.ReplaceAll(selection(), catalog().ExpectedName, "Other"),
		"duplicate":           selection() + selection(),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := prepareForm([]byte(form), catalog()); err == nil {
				t.Fatal("accepted changed form")
			}
		})
	}
	for _, form := range []string{strings.ReplaceAll(manager, `style="display:none;"`, ""), strings.ReplaceAll(manager, `action=""`, `action="https://elsewhere.invalid/"`), strings.ReplaceAll(manager, `name="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"`, `name="unknown"`), "<html>login required</html>", strings.Replace(manager, `name="bbbb`, `style="visibility:hidden" name="bbbb`, 1)} {
		if _, err := downloadForm([]byte(form)); err == nil {
			t.Fatal("accepted ambiguous manager")
		}
	}
	for _, location := range []string{"https://elsewhere.invalid/index.php?page=manager&s=49&download=1", "/index.php?page=manager&s=24&download=1", "/index.php?page=manager&s=49&download=1&s=49", "/index.php?page=manager&s=49&download=1#fragment", "/other?page=manager&s=49&download=1"} {
		if _, err := managerURL("https://datomatic.no-intro.org", location, "49"); err == nil {
			t.Fatal("accepted redirect", location)
		}
	}
}
func TestDocumentValidation(t *testing.T) {
	good := document()
	if _, _, err := Validate(good, catalog()); err != nil {
		t.Fatal(err)
	}
	for _, b := range [][]byte{
		bytes.Replace(good, []byte("<id>49</id>"), []byte("<id>24</id>"), 1),
		bytes.Replace(good, []byte("<id>49</id>"), []byte("<id>49</id><id>49</id>"), 1),
		bytes.Replace(good, []byte("01234567"), []byte("invalid!"), 1),
		bytes.Replace(good, []byte(`size="1"`), []byte(`size="-1"`), 1),
		bytes.Replace(good, []byte(`status="nodump"`), nil, 1),
		bytes.Replace(good, []byte("</datafile>"), []byte(`<game name="Empty"/></datafile>`), 1),
	} {
		if _, _, err := Validate(b, catalog()); err == nil {
			t.Fatal("invalid structure accepted")
		}
	}
	c := catalog()
	c.Validation.MinimumGames = 3
	if _, _, err := Validate(good, c); err == nil {
		t.Fatal("floor ignored")
	}
}
func TestBoundsCancellationAndBackoff(t *testing.T) {
	for _, status := range []int{301, 302, 403, 429, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(status) }))
			defer server.Close()
			a := newAdapter(server.URL, server.Client().Transport, 0)
			r, err := a.Acquire(context.Background(), "no-intro/snes/standard", catalog(), filepath.Join(t.TempDir(), "stage"))
			if err != nil || r.Attempt.Failure == nil || requests != 1 {
				t.Fatal(r, err, requests)
			}
			if status == 429 || status == 503 {
				if r.Attempt.RetryAt == nil {
					t.Fatal("missing default backoff")
				}
			}
		})
	}
	a := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.next = time.Now().Add(time.Second)
	if r, err := a.Acquire(ctx, "no-intro/snes/standard", catalog(), filepath.Join(t.TempDir(), "stage")); err == nil && r.Attempt.Failure == nil {
		t.Fatal("cancellation ignored")
	}
	for _, value := range []string{"999999999999999999999999999999", "86400", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat), "invalid"} {
		if !retryAt(value, time.Now()).After(time.Now()) {
			t.Fatal("bad retry deadline")
		}
	}
}

func TestHTTPBoundariesAndPacing(t *testing.T) {
	for _, kind := range []string{"oversize-form", "encoding", "truncated", "oversize-download", "html-download"} {
		t.Run(kind, func(t *testing.T) {
			step := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				step++
				switch {
				case kind == "oversize-form":
					w.Header().Set("Content-Length", fmt.Sprint(formLimit+1))
				case kind == "encoding":
					w.Header().Set("Content-Encoding", "gzip")
				case kind == "truncated":
					w.Header().Set("Content-Length", "100")
					io.WriteString(w, "short")
				case step == 1:
					io.WriteString(w, selection())
				case step == 2:
					w.Header().Set("Location", "/index.php?page=manager&s=49&download=1")
					w.WriteHeader(302)
				case step == 3:
					io.WriteString(w, manager)
				case kind == "oversize-download":
					w.Header().Set("Content-Length", fmt.Sprint(publisher.MaxInput+1))
				default:
					io.WriteString(w, "<html>not a catalog</html>")
				}
			}))
			defer server.Close()
			stage := filepath.Join(t.TempDir(), "input")
			r, err := newAdapter(server.URL, server.Client().Transport, 0).Acquire(context.Background(), "no-intro/snes/standard", catalog(), stage)
			if err != nil || r.Attempt.Failure == nil || r.Attempt.Path != nil {
				t.Fatal("accepted HTTP failure", r, err)
			}
			entries, err := os.ReadDir(stage)
			if err != nil || len(entries) != 0 {
				t.Fatal("failed response escaped staging", err)
			}
		})
	}
	var observed []time.Time
	step := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed = append(observed, time.Now())
		step++
		switch step {
		case 1:
			io.WriteString(w, selection())
		case 2:
			w.Header().Set("Location", "/index.php?page=manager&s=49&download=1")
			w.WriteHeader(302)
		case 3:
			io.WriteString(w, manager)
		default:
			w.Write(document())
		}
	}))
	defer server.Close()
	gap := 20 * time.Millisecond
	r, err := newAdapter(server.URL, server.Client().Transport, gap).Acquire(context.Background(), "no-intro/snes/standard", catalog(), filepath.Join(t.TempDir(), "input"))
	if err != nil || r.Code != "" || len(observed) != 4 {
		t.Fatal(r, err, len(observed))
	}
	for i := 1; i < len(observed); i++ {
		if observed[i].Sub(observed[i-1]) < gap-time.Millisecond {
			t.Fatal("request pacing ignored")
		}
	}
	a := New()
	if a.gap != 5*time.Second || a.client.Timeout != 30*time.Second {
		t.Fatal("production bounds changed")
	}
}
