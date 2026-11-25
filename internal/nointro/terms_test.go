package nointro

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

const reviewedTerms = "<html>Operator-reviewed terms for this fixture</html>"

func TestTokenBoundSelectionPreservesCompleteExport(t *testing.T) {
	token := strings.Repeat("a", 64)
	control := `<input type="hidden" name="csrf_token" value="` + token + `">`
	form := strings.Replace(selection(), "download_dat_2026-09-05", "download_request", 1)
	form = strings.Replace(form, "</form>", control+"</form>", 1)
	got, err := prepareForm([]byte(form), catalog())
	if err != nil {
		t.Fatal(err)
	}
	want := completeSelection()
	want.Set("download_request", "Prepare")
	want.Set("csrf_token", token)
	if got.Encode() != want.Encode() {
		t.Fatal("token-bound selector changed the complete export policy")
	}
	for _, invalid := range []string{
		strings.Replace(form, control, "", 1),
		strings.Replace(form, control, control+control, 1),
		strings.Replace(form, token, "malformed", 1),
		strings.Replace(form, control, strings.Replace(control, "csrf_token", "another_token", 1), 1),
	} {
		if _, err := prepareForm([]byte(invalid), catalog()); err == nil {
			t.Fatal("invalid token-bound selector was accepted")
		}
	}
}

func TestTokenBoundDownloadUsesOnlyTheManagerControls(t *testing.T) {
	token := strings.Repeat("b", 64)
	control := `<input name="csrf_token" type="hidden" value="` + token + `">`
	form := `<form action="" method="post">` + control + `<input type="submit" name="download_file" value="Download!!"></form>`
	got, err := downloadForm([]byte(form))
	if err != nil || len(got) != 2 || got.Get("csrf_token") != token || got.Get("download_file") != "Download!!" {
		t.Fatal("manager controls were not preserved", err)
	}
	for _, invalid := range []string{
		strings.Replace(form, control, "", 1),
		strings.Replace(form, control, control+control, 1),
		strings.Replace(form, token, "malformed", 1),
		strings.Replace(form, `type="hidden"`, `type="hidden" disabled`, 1),
	} {
		if _, err := downloadForm([]byte(invalid)); err == nil {
			t.Fatal("invalid token-bound manager form was accepted")
		}
	}
}

func consentPage(token string) string {
	return `<p>To use DAT-o-MATIC, please read and accept our Terms and Conditions, which include the Privacy Policy.
<a href="terms.html" target="_blank">Read the terms</a></p>
<form method="post" action="" accept-charset="UTF-8">
<input type="hidden" name="csrf_token" value="` + token + `">
<input type="submit" name="terms_accept" value="I accept">
</form>`
}

func TestDatomaticConsentIsScopedToApprovedTermsAndAnonymousSession(t *testing.T) {
	token := strings.Repeat("a", 64)
	digest := publisher.Hash([]byte(reviewedTerms))
	for _, tc := range []struct {
		name, approval, failure string
		documents, accepts      int32
	}{
		{name: "missing approval", failure: "terms_acceptance_required"},
		{name: "changed terms", approval: strings.Repeat("b", 64), failure: "terms_changed", documents: 1},
		{name: "cross-origin response", approval: digest, failure: "unexpected_redirect", documents: 1, accepts: 1},
		{name: "rejected consent", approval: digest, failure: "terms_acceptance_rejected", documents: 1, accepts: 1},
		{name: "matching approval", approval: digest, documents: 1, accepts: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var documents, accepts, prompts, downloads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/terms.html" {
					documents.Add(1)
					io.WriteString(w, reviewedTerms)
					return
				}
				if r.Method == http.MethodPost {
					if err := r.ParseForm(); err != nil {
						t.Errorf("read form: %v", err)
					}
					if r.Form.Has("terms_accept") {
						accepts.Add(1)
						session, err := r.Cookie("session")
						if err != nil || session.Value != "anonymous" || r.Form.Get("csrf_token") != token {
							t.Error("acceptance must use the original anonymous session and token")
						}
						switch tc.name {
						case "cross-origin response":
							http.Redirect(w, r, "https://unrelated.invalid/index.php", http.StatusFound)
						case "rejected consent":
							io.WriteString(w, consentPage(token))
						default:
							http.SetCookie(w, &http.Cookie{Name: "session", Value: "accepted", Path: "/"})
							http.Redirect(w, r, "/index.php?page=download&op=dat&s=49", http.StatusFound)
						}
						return
					}
				}
				session, err := r.Cookie("session")
				if err != nil || session.Value != "accepted" {
					prompts.Add(1)
					http.SetCookie(w, &http.Cookie{Name: "session", Value: "anonymous", Path: "/"})
					io.WriteString(w, consentPage(token))
					return
				}
				switch {
				case r.URL.Query().Get("page") == "manager" && r.Method == http.MethodPost:
					downloads.Add(1)
					w.Write(document())
				case r.URL.Query().Get("page") == "manager":
					io.WriteString(w, manager)
				case r.Method == http.MethodPost:
					http.Redirect(w, r, "/index.php?page=manager&s=49&download=1", http.StatusFound)
				default:
					io.WriteString(w, selection())
				}
			}))
			defer server.Close()
			a := newAdapter(server.URL, server.Client().Transport, 0)
			if err := a.setTermsApproval(tc.approval); err != nil {
				t.Fatal(err)
			}
			acquisitions := 1
			if tc.failure == "" || tc.failure == "terms_acceptance_required" || tc.failure == "terms_changed" {
				acquisitions = 2
			}
			for i := 0; i < acquisitions; i++ {
				result, err := a.Acquire(context.Background(), "no-intro/snes/standard", catalog(), filepath.Join(t.TempDir(), "input"))
				if err != nil || result.Code != tc.failure {
					t.Fatalf("got outcome %q, expected %q: %v", result.Code, tc.failure, err)
				}
				if (result.Attempt.Path != nil) != (tc.failure == "") {
					t.Fatal("candidate availability did not match acquisition outcome")
				}
			}
			if documents.Load() != tc.documents || accepts.Load() != tc.accepts {
				t.Fatalf("got %d terms reads, %d acceptances", documents.Load(), accepts.Load())
			}
			if prompts.Load() != 1 {
				t.Fatalf("expected one visit to the terms prompt, got %d", prompts.Load())
			}
			if count := downloads.Load(); (tc.failure == "" && count != 2) || (tc.failure != "" && count != 0) {
				t.Fatalf("unexpected DAT download count %d", count)
			}
		})
	}
}
