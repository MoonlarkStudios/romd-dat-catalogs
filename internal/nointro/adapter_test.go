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
	"regexp"
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

func TestMissingInActionSelection(t *testing.T) {
	for _, controls := range []string{
		`<input type="radio" name="inc_mia" value="1"><input type="radio" name="inc_mia" value="2"><input type="radio" name="inc_mia" value="0">`,
		`<input type="radio" name="inc_mia" value="0">`,
		`<input type="radio" name="inc_mia" value="1"><input type="radio" name="inc_mia" value="1">`,
		`<input type="hidden" name="inc_mia" value="1">`,
	} {
		raw := strings.Replace(selection(), "</form>", controls+"</form>", 1)
		values, err := prepareForm([]byte(raw), catalog())
		if strings.Contains(controls, `value="2"`) {
			if err != nil || values.Get("inc_mia") != "1" {
				t.Fatal(values, err)
			}
		} else if err == nil {
			t.Fatal("accepted ambiguous or incomplete MIA controls")
		}
	}
}

func TestReviewedHandheldForms(t *testing.T) {
	for _, tc := range []struct{ key, id, name string }{{"gb", "46", "Nintendo - Game Boy"}, {"gbc", "47", "Nintendo - Game Boy Color"}, {"gba", "23", "Nintendo - Game Boy Advance"}} {
		t.Run(tc.key, func(t *testing.T) {
			c := catalog()
			c.SystemID = tc.key
			c.ProviderSystemID = tc.id
			c.ExpectedName = tc.name
			raw := strings.ReplaceAll(strings.ReplaceAll(selection(), "49", tc.id), catalog().ExpectedName, tc.name)
			if tc.key == "gbc" {
				raw = regexp.MustCompile(`<input[^>]*name="collection"[^>]*>`).ReplaceAllString(raw, "")
			}
			if tc.key == "gba" {
				raw = regexp.MustCompile(`<input[^>]*name="inc_adult"[^>]*>`).ReplaceAllString(raw, "")
				raw = strings.Replace(raw, "</form>", `<input type="radio" name="numbered" value="0"><input type="radio" name="inc_xroms" value="1"><input type="radio" name="inc_zroms" value="1"></form>`, 1)
			}
			values, err := prepareForm([]byte(raw), c)
			if err != nil {
				t.Fatal(err)
			}
			if tc.key == "gbc" && values.Has("collection") {
				t.Fatal("invented collection control")
			}
			if tc.key == "gba" && (values.Get("numbered") != "0" || values.Get("inc_xroms") != "1" || values.Get("inc_zroms") != "1" || values.Has("inc_adult")) {
				t.Fatal(values)
			}
			if _, err := prepareForm([]byte(raw), catalog()); err == nil {
				t.Fatal("cross-system form accepted")
			}
			doc := bytes.ReplaceAll(bytes.ReplaceAll(document(), []byte("<id>49</id>"), []byte("<id>"+tc.id+"</id>")), []byte(catalog().ExpectedName), []byte(tc.name))
			if _, _, err := Validate(doc, c); err != nil {
				t.Fatal(err)
			}
			if _, _, err := Validate(doc, catalog()); err == nil {
				t.Fatal("cross-system DAT accepted")
			}
		})
	}
}

func TestPacingAcrossCatalogs(t *testing.T) {
	var starts []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		starts = append(starts, time.Now())
		id := r.URL.Query().Get("s")
		name := "Nintendo - Game Boy"
		if id == "49" {
			name = catalog().ExpectedName
		}
		switch (len(starts) - 1) % 4 {
		case 0:
			io.WriteString(w, strings.ReplaceAll(strings.ReplaceAll(selection(), "49", id), catalog().ExpectedName, name))
		case 1:
			w.Header().Set("Location", "/index.php?page=manager&s="+id+"&download=1")
			w.WriteHeader(302)
		case 2:
			io.WriteString(w, manager)
		case 3:
			w.Write(bytes.ReplaceAll(bytes.ReplaceAll(document(), []byte("<id>49</id>"), []byte("<id>"+id+"</id>")), []byte(catalog().ExpectedName), []byte(name)))
		}
	}))
	defer server.Close()
	gap := 20 * time.Millisecond
	a := newAdapter(server.URL, server.Client().Transport, gap)
	for _, key := range []string{"snes", "gb"} {
		c := catalog()
		if key == "gb" {
			c.SystemID = "gb"
			c.ProviderSystemID = "46"
			c.ExpectedName = "Nintendo - Game Boy"
		}
		result, err := a.Acquire(context.Background(), "no-intro/"+key+"/standard", c, filepath.Join(t.TempDir(), "input"))
		if err != nil || result.Code != "" {
			t.Fatal(result, err)
		}
	}
	if len(starts) != 8 || starts[4].Sub(starts[3]) < gap-time.Millisecond {
		t.Fatal("cross-catalog request pacing lost")
	}
}

func TestHomeConsoleSelections(t *testing.T) {
	for _, tc := range []struct{ key, id, name string }{
		{"nes", "45", "Nintendo - Nintendo Entertainment System (Headered)"},
		{"genesis", "32", "Sega - Mega Drive - Genesis"},
		{"n64", "24", "Nintendo - Nintendo 64 (BigEndian)"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			c := catalog()
			c.SystemID, c.ProviderSystemID, c.ExpectedName = tc.key, tc.id, tc.name
			raw := strings.ReplaceAll(strings.ReplaceAll(selection(), "49", tc.id), catalog().ExpectedName, tc.name)
			raw = regexp.MustCompile(`<input[^>]*name="collection"[^>]*>`).ReplaceAllString(raw, "")
			if tc.key == "nes" {
				raw = strings.ReplaceAll(raw, tc.name, "Nintendo - Nintendo Entertainment System")
				raw = strings.Replace(raw, "</form>", `<input type="radio" name="header_plugin" value="0"><input type="radio" name="header_plugin" value="1"></form>`, 1)
			}
			if tc.key == "n64" {
				raw = strings.ReplaceAll(raw, tc.name, "Nintendo - Nintendo 64")
				raw = regexp.MustCompile(`<input[^>]*name="inc_adult"[^>]*>`).ReplaceAllString(raw, "")
			}
			values, err := prepareForm([]byte(raw), c)
			if err != nil || values.Get("format") != "0" || values.Has("collection") {
				t.Fatal("wrong reviewed representation", values, err)
			}
			if tc.key == "nes" && values.Get("header_plugin") != "0" {
				t.Fatal("NES header plugin enabled")
			}
			if tc.key == "n64" && values.Has("inc_adult") {
				t.Fatal("invented N64 control")
			}
			for _, broken := range []string{
				regexp.MustCompile(`<input[^>]*name="format"[^>]*>`).ReplaceAllString(raw, `<input type="radio" name="format" value="1">`),
				strings.Replace(raw, "</form>", `<input type="radio" name="format" value="0"></form>`, 1),
			} {
				if _, err := prepareForm([]byte(broken), c); err == nil {
					t.Fatal("missing or ambiguous representation accepted")
				}
			}
			if _, err := prepareForm([]byte(raw), catalog()); err == nil {
				t.Fatal("wrong system form accepted")
			}
			doc := bytes.ReplaceAll(bytes.ReplaceAll(document(), []byte("<id>49</id>"), []byte("<id>"+tc.id+"</id>")), []byte(catalog().ExpectedName), []byte(tc.name))
			if _, _, err := Validate(doc, c); err != nil {
				t.Fatal(err)
			}
			if tc.key == "nes" || tc.key == "n64" {
				wrong := bytes.ReplaceAll(bytes.ReplaceAll(doc, []byte("(Headered)"), []byte("(Headerless)")), []byte("(BigEndian)"), []byte("(ByteSwapped)"))
				if _, _, err := Validate(wrong, c); err == nil {
					t.Fatal("wrong representation accepted")
				}
			}
			if _, _, err := Validate(doc, catalog()); err == nil {
				t.Fatal("wrong system document accepted")
			}
		})
	}
}

func TestReviewedBatchThreeForms(t *testing.T) {
	for _, tc := range []struct{ key, id, name string }{
		{"sms", "26", "Sega - Master System - Mark III"},
		{"gg", "25", "Sega - Game Gear"},
		{"tg16", "12", "NEC - PC Engine - TurboGrafx-16"},
		{"32x", "17", "Sega - 32X"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			c := catalog()
			c.SystemID, c.ProviderSystemID, c.ExpectedName = tc.key, tc.id, tc.name
			raw := strings.ReplaceAll(strings.ReplaceAll(selection(), "49", tc.id), catalog().ExpectedName, tc.name)
			for _, control := range []string{"collection", "inc_adult"} {
				raw = regexp.MustCompile(`<input[^>]*name="`+control+`"[^>]*>`).ReplaceAllString(raw, "")
			}
			if tc.key != "sms" {
				raw = regexp.MustCompile(`<input[^>]*name="inc_nodump"[^>]*>`).ReplaceAllString(raw, "")
			}
			if tc.key == "sms" || tc.key == "gg" {
				raw = strings.Replace(raw, "</form>", `<input type="radio" name="inc_xroms" value="1"><input type="radio" name="inc_xroms" value="0"></form>`, 1)
			}
			if tc.key == "sms" {
				raw = strings.Replace(raw, "</form>", `<input type="radio" name="inc_zroms" value="1"><input type="radio" name="inc_zroms" value="0"></form>`, 1)
			}
			if tc.key == "gg" || tc.key == "tg16" {
				raw = strings.Replace(raw, "</form>", `<input type="radio" name="inc_mia" value="1"></form>`, 1)
			}
			values, err := prepareForm([]byte(raw), c)
			if err != nil || values.Get("format") != "0" || values.Has("collection") || values.Has("inc_adult") {
				t.Fatal(values, err)
			}
			if (tc.key == "sms" || tc.key == "gg") && values.Get("inc_xroms") != "1" {
				t.Fatal("excluded x-ROM group")
			}
			if tc.key == "sms" && (values.Get("inc_zroms") != "1" || values.Get("inc_nodump") != "1") {
				t.Fatal("excluded SMS category")
			}
			if tc.key != "sms" && values.Has("inc_nodump") {
				t.Fatal("invented absent nodump control")
			}
			if (tc.key == "gg" || tc.key == "tg16") && values.Get("inc_mia") != "1" {
				t.Fatal("excluded MIA")
			}
			for _, control := range []string{"format", "release_2", "license_0", "storage_2"} {
				broken := regexp.MustCompile(`<input[^>]*name="`+control+`"[^>]*>`).ReplaceAllString(raw, "")
				if _, err := prepareForm([]byte(broken), c); err == nil {
					t.Fatal("missing required selection accepted", control)
				}
			}
			if tc.key == "sms" || tc.key == "gg" {
				broken := strings.Replace(raw, `name="inc_xroms" value="1"`, `name="inc_xroms" value="0"`, 1)
				if _, err := prepareForm([]byte(broken), c); err == nil {
					t.Fatal("missing inclusion accepted")
				}
			}
			if _, err := prepareForm([]byte(raw), catalog()); err == nil {
				t.Fatal("cross-system form accepted")
			}
			doc := bytes.ReplaceAll(bytes.ReplaceAll(document(), []byte("<id>49</id>"), []byte("<id>"+tc.id+"</id>")), []byte(catalog().ExpectedName), []byte(tc.name))
			if _, _, err := Validate(doc, c); err != nil {
				t.Fatal(err)
			}
			if _, _, err := Validate(doc, catalog()); err == nil {
				t.Fatal("cross-system DAT accepted")
			}
		})
	}
}
