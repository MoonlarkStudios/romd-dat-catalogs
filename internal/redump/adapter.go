// Package redump acquires an explicit, reviewed catalog allowlist. It is not
// connected to the scheduled publisher: production source enablement is separate.
package redump

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

const MaxCatalogs = 25

var systemID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var catalogID = regexp.MustCompile(`^redump/[a-z0-9]+(?:[/-][a-z0-9]+)*$`)

// Catalog is operator-reviewed configuration, never populated from remote HTML.
// Count floors are explicit anomaly gates, not proof of catalog completeness.
type Catalog struct {
	ID, System, ExpectedName, Platform, Representation, PolicyVersion string
	MinGames, MinROMs                                                 int
}
type Result struct {
	Catalog Catalog
	Attempt publisher.Attempt
	SHA256  string
	Counts  publisher.Counts
	Code    string
	RetryAt time.Time
}
type Adapter struct {
	origin   string
	client   *http.Client
	gate     chan struct{}
	cooldown time.Time
}

func New() *Adapter { return newAdapter("https://redump.org", http.DefaultTransport) }
func newAdapter(origin string, transport http.RoundTripper) *Adapter {
	return &Adapter{origin: origin, gate: make(chan struct{}, 1), client: &http.Client{Transport: transport, Timeout: 30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

// Acquire makes at most one request per catalog, sequentially, with a two-minute
// batch budget. Results contain publisher attempts pointing into a new staging
// directory; the caller owns its lifetime. No existing publication is modified.
// 429/503 stop the batch and set a cooldown shared by subsequent calls on this
// adapter. Calls are serialized with cancellable admission. Use Scheduler for
// durable admission across processes; live polling requires its recovery wiring.
func (a *Adapter) Acquire(ctx context.Context, catalogs []Catalog, stage string) ([]Result, error) {
	return a.acquire(ctx, catalogs, stage, nil)
}
func (a *Adapter) acquire(ctx context.Context, catalogs []Catalog, stage string, observe func(Result) error) ([]Result, error) {
	if a.client == nil || a.gate == nil {
		return nil, errors.New("adapter must be constructed with New")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	select {
	case a.gate <- struct{}{}:
		defer func() { <-a.gate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if e := validateCatalogs(catalogs); e != nil {
		return nil, e
	}
	origin, e := url.Parse(a.origin)
	if e != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" || origin.Path != "" {
		return nil, errors.New("HTTPS origin required")
	}
	// Mkdir, unlike MkdirAll, rejects reuse of an existing staging directory.
	if e = os.Mkdir(stage, 0700); e != nil {
		return nil, e
	}
	results := make([]Result, 0, len(catalogs))
	cooldown := a.cooldown
	stopped := time.Now().Before(cooldown)
	for i, c := range catalogs {
		r := Result{Catalog: c, Attempt: publisher.Attempt{CatalogID: c.ID, ExpectedName: c.ExpectedName, SourceURL: a.origin + "/datfile/" + c.System + "/"}}
		var raw []byte
		if stopped {
			r.Code = "provider_backoff"
			r.RetryAt = cooldown
		} else if ctx.Err() != nil {
			r.Code = "cancelled"
		} else {
			raw, r.Code, r.RetryAt = a.fetch(ctx, r.Attempt.SourceURL)
			if r.Code == "rate_limited" || r.Code == "unavailable" {
				stopped = true
				cooldown = r.RetryAt
				a.cooldown = cooldown
			}
			if r.Code == "" {
				var document []byte
				document, r.Counts, e = publisher.Document(raw, c.ExpectedName)
				if e != nil {
					r.Code = "invalid_document"
				} else if r.Counts.Games < c.MinGames || r.Counts.ROMs < c.MinROMs {
					r.Code = "count_below_floor"
				} else {
					r.SHA256 = publisher.Hash(document)
				}
			}
		}
		if observe != nil {
			if e = observe(r); e != nil {
				return nil, e
			}
		}
		if r.Code == "" {
			path := filepath.Join(stage, strconv.Itoa(i)+".input")
			if e = os.WriteFile(path, raw, 0600); e != nil {
				return nil, e
			}
			path, e = filepath.Abs(path)
			if e != nil {
				return nil, e
			}
			r.Attempt.Path = &path
		} else {
			code := r.Code
			r.Attempt.Failure = &code
		}
		results = append(results, r)
	}
	return results, nil
}

func (a *Adapter) fetch(ctx context.Context, source string) ([]byte, string, time.Time) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if e != nil {
		return nil, "request_failed", time.Time{}
	}
	req.Header.Set("User-Agent", "ROMD-DAT-Catalogs/experimental (+https://github.com/MoonlarkStudios/romd-dat-catalogs)")
	req.Header.Set("Accept-Encoding", "identity")
	response, e := a.client.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return nil, "cancelled", time.Time{}
		}
		return nil, "request_failed", time.Time{}
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusTooManyRequests:
		return nil, "rate_limited", retryAt(response.Header.Get("Retry-After"), time.Now())
	case http.StatusServiceUnavailable:
		return nil, "unavailable", retryAt(response.Header.Get("Retry-After"), time.Now())
	case http.StatusOK:
	default:
		return nil, "http_status", time.Time{}
	}
	if encoding := response.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		return nil, "unexpected_encoding", time.Time{}
	}
	if response.ContentLength > publisher.MaxInput {
		return nil, "input_limit", time.Time{}
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, publisher.MaxInput+1))
	if e != nil {
		return nil, "incomplete_response", time.Time{}
	}
	if len(raw) > publisher.MaxInput {
		return nil, "input_limit", time.Time{}
	}
	return raw, "", time.Time{}
}
func retryAt(value string, now time.Time) time.Time {
	if value != "" && strings.Trim(value, "0123456789") == "" {
		seconds, e := strconv.ParseUint(value, 10, 64)
		ceiling := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
		if e != nil || seconds > uint64(ceiling.Unix()-now.Unix()) {
			return ceiling
		}
		return time.Unix(now.Unix()+int64(seconds), int64(now.Nanosecond()))
	}
	if date, e := http.ParseTime(value); e == nil && date.After(now) {
		return date
	}
	return now.Add(time.Minute)
}

func validateCatalogs(catalogs []Catalog) error {
	if len(catalogs) == 0 || len(catalogs) > MaxCatalogs {
		return errors.New("catalog count must be 1..25")
	}
	seen, systems := map[string]bool{}, map[string]bool{}
	for _, c := range catalogs {
		if !catalogID.MatchString(c.ID) || !systemID.MatchString(c.System) || seen[c.ID] || systems[c.System] || strings.TrimSpace(c.ExpectedName) == "" || strings.TrimSpace(c.Platform) == "" || strings.TrimSpace(c.Representation) == "" || strings.TrimSpace(c.PolicyVersion) == "" || c.MinGames < 1 || c.MinROMs < 1 {
			return errors.New("invalid or duplicate reviewed catalog identity")
		}
		seen[c.ID], systems[c.System] = true, true
	}
	return nil
}
