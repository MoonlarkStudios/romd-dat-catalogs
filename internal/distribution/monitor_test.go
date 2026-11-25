package distribution

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/definitions"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

const monitorCatalog = "synthetic/console/standard"
const monitorName = "ROMD Synthetic Console"

func monitorStage(t *testing.T, f *fixture, version int64, now time.Time) string {
	t.Helper()
	registry, err := definitions.LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(f.dir, fmt.Sprintf("monitor-stage-%d-%d", version, now.Unix()))
	_, err = StageGit(StageOptions{
		State: f.state, TrustDir: f.trust, Output: out, Keys: f.online, Version: version, Now: now, Definitions: registry,
	}, "example/data", strings.Repeat("a", 40))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func monitorAssetClient(body []byte) *http.Client {
	return &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
}

func monitorCheck(t *testing.T, f *fixture, metadataClient, assetClient *http.Client) MirrorReport {
	t.Helper()
	report, err := MonitorMirror(f.root, f.server.URL+"/site", filepath.Join(f.dir, "monitor-cache"),
		monitorCatalog, monitorName, metadataClient, assetClient)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestMonitorRecoversFromBrief503AndAlertsOnRepeatedFailure(t *testing.T) {
	f := makeFixture(t)
	f.location.Store(monitorStage(t, f, 1, f.now))
	assets := monitorAssetClient(read(t, "../../fixtures/example.dat"))
	rootProbes := 0
	offline := false
	client := mirrorTestClient(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/2.root.json") {
			rootProbes++
			if offline || rootProbes <= 3 {
				return mirrorResponse(http.StatusServiceUnavailable), nil
			}
		}
		return http.DefaultTransport.RoundTrip(r)
	})
	first := monitorCheck(t, f, client, assets)
	if rootProbes != 4 || !first.Verified || first.Alert != "" || first.ConsecutiveFailures != 0 {
		t.Fatalf("short outage must recover during the first check: %+v", first)
	}

	offline = true
	missed := monitorCheck(t, f, client, assets)
	if missed.Verified || missed.Alert != "" || missed.ConsecutiveFailures != 1 ||
		missed.LastVerified == nil || missed.LastVerified.Version != 1 {
		t.Fatalf("first exhausted check must preserve the last verified timestamp: %+v", missed)
	}
	repeated := monitorCheck(t, f, client, assets)
	if repeated.Verified || repeated.ConsecutiveFailures != 2 || repeated.Alert != "repeated_verification_failure" {
		t.Fatalf("second exhausted check must alert: %+v", repeated)
	}
	offline = false
	recovered := monitorCheck(t, f, client, assets)
	if !recovered.Verified || recovered.Alert != "" || recovered.ConsecutiveFailures != 0 {
		t.Fatalf("successful verification must clear the incident: %+v", recovered)
	}
}

func TestMonitorAlertsBeforeExpiryAndStillRejectsExpiredMetadata(t *testing.T) {
	f := makeFixture(t)
	f.location.Store(monitorStage(t, f, 1, f.now.Add(-37*time.Hour)))
	assets := monitorAssetClient(read(t, "../../fixtures/example.dat"))
	report := monitorCheck(t, f, f.server.Client(), assets)
	if !report.Verified || report.Alert != "metadata_expiring" || report.SecondsUntilExpiry == nil ||
		*report.SecondsUntilExpiry <= 0 || *report.SecondsUntilExpiry > int64(mirrorExpiryWarning.Seconds()) {
		t.Fatalf("a valid but expiring publication must alert while still usable: %+v", report)
	}
	unavailable := mirrorTestClient(func(*http.Request) (*http.Response, error) {
		return mirrorResponse(http.StatusServiceUnavailable), nil
	})
	report = monitorCheck(t, f, unavailable, assets)
	if report.Verified || report.Alert != "metadata_expiring" || report.ConsecutiveFailures != 1 {
		t.Fatalf("one failed check must still alert near the last trusted expiry: %+v", report)
	}

	f.location.Store(monitorStage(t, f, 2, f.now.Add(-49*time.Hour)))
	report = monitorCheck(t, f, f.server.Client(), assets)
	if report.Verified || report.Alert != "metadata_expired" || report.LastVerified == nil || report.LastVerified.Version != 1 {
		t.Fatalf("expired metadata must fail closed and preserve the last verified publication: %+v", report)
	}
}

func TestMonitorSeparatesUpstreamFailureFromCandidateVerification(t *testing.T) {
	f := makeFixture(t)
	failure := "fixture acquisition failure"
	_, err := publisher.Publish(f.state, []publisher.Attempt{{
		CatalogID: monitorCatalog, ExpectedName: monitorName, SourceURL: "https://example.invalid/dat", Failure: &failure,
	}}, "https://catalogs.example.invalid/", publisher.Options{Now: f.now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	f.location.Store(monitorStage(t, f, 2, f.now))
	assets := monitorAssetClient(read(t, "../../fixtures/example.dat"))
	report := monitorCheck(t, f, f.server.Client(), assets)
	if !report.Verified || report.Alert != "" || report.UpstreamHealth != "failed" || report.LastUpstreamSuccess == nil {
		t.Fatalf("retained authenticated asset and upstream failure must remain distinct: %+v", report)
	}
	report = monitorCheck(t, f, f.server.Client(), monitorAssetClient([]byte("altered")))
	if report.Verified || report.Failure != "candidate_verification_failed" || report.LastVerified.Version != 2 {
		t.Fatalf("altered asset must fail while the directory is still authenticated: %+v", report)
	}

	f.location.Store(monitorStage(t, f, 1, f.now.Add(-time.Minute)))
	report = monitorCheck(t, f, f.server.Client(), assets)
	if report.Verified || report.Alert != "repeated_verification_failure" || report.LastVerified.Version != 2 {
		t.Fatalf("rollback must fail and preserve the last verified publication: %+v", report)
	}
}
