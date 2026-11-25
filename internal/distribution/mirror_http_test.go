package distribution

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestMirrorRootProbe(t *testing.T) {
	for _, test := range []struct {
		name       string
		unhealthy  int
		wantProbes int
	}{
		{name: "missing root", wantProbes: 1},
		{name: "temporarily unavailable", unhealthy: 3, wantProbes: 4},
		{name: "stays unavailable", unhealthy: 4, wantProbes: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := makeFixture(t)
			expected, stage := f.stage(t, 1)
			f.location.Store(stage)
			probes := 0
			client := mirrorTestClient(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "/2.root.json") {
					probes++
					if probes <= test.unhealthy {
						return mirrorResponse(http.StatusServiceUnavailable), nil
					}
				}
				return http.DefaultTransport.RoundTrip(r)
			})
			index, err := RefreshIndex(f.root, f.server.URL+"/site", filepath.Join(f.dir, "cache"), client)
			if test.unhealthy == test.wantProbes {
				var download *metadata.ErrDownloadHTTP
				if !errors.As(err, &download) || download.StatusCode != http.StatusServiceUnavailable {
					t.Fatalf("expected the final 503 to reach TUF, got %v", err)
				}
			} else if err != nil || index.State.SHA256 != expected.State.SHA256 {
				t.Fatalf("expected the authenticated directory after the root probe, got %v", err)
			}
			if probes != test.wantProbes {
				t.Fatalf("expected %d root probes, got %d", test.wantProbes, probes)
			}
		})
	}
}

func TestMirrorRetryStillRejectsInvalidTimestamp(t *testing.T) {
	f := makeFixture(t)
	_, stage := f.stage(t, 1)
	f.location.Store(stage)
	path := filepath.Join(stage, "site/metadata/timestamp.json")
	valid := read(t, path)
	timestamp, err := metadata.Timestamp().FromBytes(valid)
	if err != nil {
		t.Fatal(err)
	}
	// Change a signed field without resigning the metadata.
	timestamp.Signed.Expires = timestamp.Signed.Expires.Add(time.Hour)
	invalid, err := timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	timestamps := 0
	client := mirrorTestClient(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/timestamp.json") {
			timestamps++
			switch timestamps {
			case 1:
				return mirrorResponse(http.StatusServiceUnavailable), nil
			case 2:
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(invalid))}, nil
			default:
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(valid))}, nil
			}
		}
		return http.DefaultTransport.RoundTrip(r)
	})
	_, err = RefreshIndex(f.root, f.server.URL+"/site", filepath.Join(f.dir, "cache"), client)
	if !errors.Is(err, &metadata.ErrUnsignedMetadata{}) {
		t.Fatalf("expected invalid signature rejection, got %v", err)
	}
	if timestamps != 2 {
		t.Fatalf("expected one HTTP retry and immediate rejection, got %d timestamp requests", timestamps)
	}
}

func TestMirrorRetryStillChecksAssetIntegrity(t *testing.T) {
	const expected = "authentic"
	asset := Asset{URL: "https://github.com/owner/repo/releases/download/tag/console.dat", Bytes: len(expected), SHA256: publisher.Hash([]byte(expected))}
	for _, body := range []string{expected, "incorrect"} {
		t.Run(body, func(t *testing.T) {
			requests := 0
			client := mirrorTestClient(func(*http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return mirrorResponse(http.StatusServiceUnavailable), nil
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			data, err := FetchAsset(asset, client)
			if body == expected && (err != nil || string(data) != expected) {
				t.Fatalf("expected authenticated bytes after retry, got %v", err)
			}
			if body != expected && err == nil {
				t.Fatal("incorrect bytes accepted after retry")
			}
			if requests != 2 {
				t.Fatalf("expected one retry, got %d requests", requests)
			}
		})
	}
}

func TestMirrorRetryAfterAndTransportErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     int
		retryAfter string
		firstError error
		wantCalls  int
	}{
		{name: "temporary network reset", firstError: syscall.ECONNRESET, wantCalls: 2},
		{name: "invalid TLS trust", firstError: x509.UnknownAuthorityError{}, wantCalls: 1},
		{name: "access denied", status: http.StatusForbidden, wantCalls: 1},
		{name: "rate limited with no requested wait", status: http.StatusTooManyRequests, retryAfter: "0", wantCalls: 2},
		{name: "requested wait exceeds budget", status: http.StatusServiceUnavailable, retryAfter: "120", wantCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			client := mirrorTestClient(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					if test.firstError != nil {
						return nil, test.firstError
					}
					res := mirrorResponse(test.status)
					res.Header.Set("Retry-After", test.retryAfter)
					return res, nil
				}
				return mirrorResponse(http.StatusOK), nil
			})
			res, err := client.Get("https://mirror.invalid")
			if res != nil {
				defer res.Body.Close()
			}
			if test.wantCalls == 2 && (err != nil || res.StatusCode != http.StatusOK) {
				t.Fatalf("expected retry to recover, got %v", err)
			}
			if calls != test.wantCalls {
				t.Fatalf("expected %d requests, got %d", test.wantCalls, calls)
			}
		})
	}
}

func TestMirrorRetryStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://mirror.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	client := &http.Client{Transport: mirrorTransport{
		base: transportFunc(func(*http.Request) (*http.Response, error) {
			calls++
			cancel()
			return mirrorResponse(http.StatusServiceUnavailable), nil
		}),
		delays: []time.Duration{time.Minute},
	}}
	if _, err := client.Do(req); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("expected immediate cancellation, got %v after %d requests", err, calls)
	}
}

func mirrorTestClient(base transportFunc) *http.Client {
	return &http.Client{Transport: mirrorTransport{base: base, delays: []time.Duration{0, 0, 0}}}
}

func mirrorResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: http.NoBody}
}
