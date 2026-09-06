// Package nointro acquires one reviewed public Standard DAT through Datomatic's
// anonymous form. It does not discover platforms, log in, or execute JavaScript.
package nointro

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/definitions"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

const formLimit = 1 << 20
const requestGap = 5 * time.Second

var number = regexp.MustCompile(`^[1-9][0-9]{0,5}$`)

type Result struct {
	Attempt publisher.Attempt
	SHA256  string
	Counts  publisher.Counts
	Code    string
}

type Adapter struct {
	origin string
	client *http.Client
	gate   chan struct{}
	next   time.Time
	gap    time.Duration
}

func SourceURL(system string) string {
	return "https://datomatic.no-intro.org/index.php?page=download&op=dat&s=" + system
}
func New() *Adapter {
	return newAdapter("https://datomatic.no-intro.org", http.DefaultTransport, requestGap)
}
func newAdapter(origin string, transport http.RoundTripper, gap time.Duration) *Adapter {
	return &Adapter{origin: origin, gap: gap, gate: make(chan struct{}, 1), client: &http.Client{Transport: transport, Timeout: 30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

// Acquire runs at most four HTTP requests, with no automatic retry, under a
// two-minute deadline. Every request shares a five-second host admission gate.
// Retry-After is returned in the existing publisher attempt for signed retention.
// Reuse one adapter per process; the CLI also honors restored retry deadlines.
func (a *Adapter) Acquire(ctx context.Context, id string, c definitions.Catalog, stage string) (Result, error) {
	r := Result{Attempt: publisher.Attempt{CatalogID: id, ExpectedName: c.ExpectedName, SourceURL: SourceURL(c.ProviderSystemID)}}
	if a.client == nil || a.gate == nil || c.Provider != "no-intro" || c.Representation != "standard" || !number.MatchString(c.ProviderSystemID) || id != "no-intro/"+c.SystemID+"/standard" || c.ExpectedName == "" || c.Validation.MinimumGames < 1 || c.Validation.MinimumROMs < 1 {
		return r, errors.New("invalid No-Intro adapter or reviewed catalog")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	select {
	case a.gate <- struct{}{}:
		defer func() { <-a.gate }()
	case <-ctx.Done():
		return r, ctx.Err()
	}
	if err := os.Mkdir(stage, 0700); err != nil {
		return r, err
	}
	// Anonymous session cookies live only for this acquisition and are never logged.
	jar, _ := cookiejar.New(nil)
	client := *a.client
	client.Jar = jar
	fail := func(code string) (Result, error) { r.Code = code; r.Attempt.Failure = &r.Code; return r, nil }
	if time.Until(a.next) > a.gap {
		stamp := a.next.UTC().Format(time.RFC3339Nano)
		r.Attempt.RetryAt = &stamp
		return fail("provider_backoff")
	}
	source := a.origin + "/index.php?page=download&op=dat&s=" + c.ProviderSystemID
	request := func(target string, values url.Values, limit int) ([]byte, *http.Response, string) {
		if delay := time.Until(a.next); delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				return nil, nil, "cancelled"
			}
		}
		method := http.MethodGet
		var body io.Reader
		if values != nil {
			method = http.MethodPost
			body = strings.NewReader(values.Encode())
		}
		req, err := http.NewRequestWithContext(ctx, method, target, body)
		if err != nil {
			return nil, nil, "request_failed"
		}
		req.Header.Set("User-Agent", "ROMD-DAT-Catalogs/experimental (+https://github.com/MoonlarkStudios/romd-dat-catalogs)")
		req.Header.Set("Accept-Encoding", "identity")
		if values != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		a.next = time.Now().Add(a.gap)
		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, "request_failed"
		}
		defer resp.Body.Close()
		if resp.StatusCode == 429 || resp.StatusCode == 503 {
			a.next = retryAt(resp.Header.Get("Retry-After"), time.Now())
			if a.next.Before(time.Now().Add(a.gap)) {
				a.next = time.Now().Add(a.gap)
			}
			stamp := a.next.UTC().Format(time.RFC3339Nano)
			r.Attempt.RetryAt = &stamp
			return nil, resp, "provider_backoff"
		}
		if resp.StatusCode != 200 && resp.StatusCode != 302 {
			return nil, resp, "http_status"
		}
		if encoding := resp.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
			return nil, resp, "unexpected_encoding"
		}
		if resp.ContentLength > int64(limit) {
			return nil, resp, "size_limit"
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
		if err != nil {
			return nil, resp, "incomplete_response"
		}
		if len(b) > limit {
			return nil, resp, "size_limit"
		}
		return b, resp, ""
	}
	form, resp, code := request(source, nil, formLimit)
	if code != "" {
		return fail(code)
	}
	if resp.StatusCode != 200 {
		return fail("unexpected_redirect")
	}
	values, err := prepareForm(form, c)
	if err != nil {
		return fail("form_changed")
	}
	_, resp, code = request(source, values, formLimit)
	if code != "" {
		return fail(code)
	}
	if resp.StatusCode != 302 {
		return fail("prepare_failed")
	}
	manager, err := managerURL(a.origin, resp.Header.Get("Location"), c.ProviderSystemID)
	if err != nil {
		return fail("unexpected_redirect")
	}
	form, resp, code = request(manager, nil, formLimit)
	if code != "" {
		return fail(code)
	}
	if resp.StatusCode != 200 {
		return fail("unexpected_redirect")
	}
	values, err = downloadForm(form)
	if err != nil {
		return fail("form_changed")
	}
	raw, resp, code := request(manager, values, publisher.MaxInput)
	if code != "" {
		return fail(code)
	}
	if resp.StatusCode != 200 {
		return fail("unexpected_redirect")
	}
	document, counts, err := Validate(raw, c)
	if err != nil {
		return fail("invalid_document")
	}
	r.Counts = counts
	r.SHA256 = publisher.Hash(document)
	path, err := filepath.Abs(filepath.Join(stage, "catalog.input"))
	if err != nil {
		return r, err
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		return r, err
	}
	r.Attempt.Path = &path
	return r, nil
}

func managerURL(origin, location, system string) (string, error) {
	base, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	u = base.ResolveReference(u)
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || u.Scheme != base.Scheme || u.Host != base.Host || u.User != nil || u.Fragment != "" || u.Path != "/index.php" || len(q) != 3 || len(q["page"]) != 1 || q.Get("page") != "manager" || len(q["s"]) != 1 || q.Get("s") != system || len(q["download"]) != 1 || !number.MatchString(q.Get("download")) {
		return "", errors.New("unexpected manager URL")
	}
	return u.String(), nil
}
func retryAt(value string, now time.Time) time.Time {
	if value != "" && strings.Trim(value, "0123456789") == "" {
		seconds, err := strconv.ParseUint(value, 10, 64)
		ceiling := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
		if err != nil || seconds > uint64(ceiling.Unix()-now.Unix()) {
			return ceiling
		}
		return time.Unix(now.Unix()+int64(seconds), int64(now.Nanosecond()))
	}
	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date
	}
	return now.Add(time.Minute)
}
