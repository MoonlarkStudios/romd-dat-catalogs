package distribution

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRestoreOrigin(t *testing.T) {
	for _, status := range []int{200, 301, 403, 404, 500} {
		client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://new.example/metadata/timestamp.json" {
				t.Fatal(r.URL)
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		})}
		origin, err := RestoreOrigin("https://new.example/", "https://old.example/", false, client)
		if status == 404 {
			if err != nil || origin != "https://old.example" {
				t.Fatalf("migration: %s %v", origin, err)
			}
		} else if err == nil {
			t.Fatalf("migration accepted HTTP %d", status)
		}
		origin, err = RestoreOrigin("https://new.example", "", true, client)
		if status == 404 {
			if err != nil || origin != "" {
				t.Fatal("bootstrap failed")
			}
		} else if err == nil {
			t.Fatalf("bootstrap accepted HTTP %d", status)
		}
	}
	offline := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })}
	if _, err := RestoreOrigin("https://new.example", "https://old.example", false, offline); err == nil {
		t.Fatal("network failure treated as absence")
	}
	origin, err := RestoreOrigin("https://new.example", "", false, offline)
	if err != nil || origin != "https://new.example" {
		t.Fatal("ordinary restore must use canonical site")
	}
	for _, args := range []struct {
		site, old string
		bootstrap bool
	}{
		{"https://new.example", "https://old.example", true},
		{"https://new.example", "https://new.example", false},
		{"http://new.example", "", true},
		{"https://new.example", "http://old.example", false},
	} {
		if _, err := RestoreOrigin(args.site, args.old, args.bootstrap, offline); err == nil {
			t.Fatal("invalid migration accepted")
		}
	}
}
