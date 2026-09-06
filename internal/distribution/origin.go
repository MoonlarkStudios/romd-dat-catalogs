package distribution

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

// RestoreOrigin never falls back from an existing or unhealthy destination to
// an older site. Migration is an explicit one-time operation using the same root.
// An empty result means an explicitly requested, demonstrably absent new site.
func RestoreOrigin(site, previous string, bootstrap bool, client *http.Client) (string, error) {
	site = strings.TrimRight(site, "/")
	previous = strings.TrimRight(previous, "/")
	if !strings.HasPrefix(site, "https://") || (previous != "" && !strings.HasPrefix(previous, "https://")) {
		return "", errors.New("HTTPS publication sites required")
	}
	if bootstrap && previous != "" {
		return "", errors.New("bootstrap and migration are exclusive")
	}
	if !bootstrap && previous == "" {
		return site, nil
	}
	if site == previous {
		return "", errors.New("migration requires a different destination")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	probe := *client
	probe.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := probe.Get(site + "/metadata/timestamp.json")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		return "", errors.New("bootstrap/migration requires destination timestamp HTTP 404")
	}
	return previous, nil
}
