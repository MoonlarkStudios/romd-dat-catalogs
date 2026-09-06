// Package checkpoint provides authoritative source-state custody for ephemeral
// runners. Credentials are transport-only and never included in checkpoint data.
package checkpoint

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MaxBytes = 512 << 10
const Branch = "source-state"
const File = "redump-state.json"

var ErrConflict = errors.New("checkpoint changed; reload and review before retrying")
var repository = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)
var revision = regexp.MustCompile(`^[a-f0-9]{40}$`)

type GitHub struct {
	endpoint, token string
	client          *http.Client
}

func NewGitHub(repo, token string) (*GitHub, error) {
	if !repository.MatchString(repo) || strings.TrimSpace(token) == "" {
		return nil, errors.New("repository and GitHub token required")
	}
	return &GitHub{endpoint: "https://api.github.com/repos/" + repo + "/contents/" + File, token: token, client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func blobSHA(raw []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d%c", len(raw), 0)
	h.Write(raw)
	return fmt.Sprintf("%x", h.Sum(nil))
}
func (g *GitHub) request(ctx context.Context, method string, body []byte) ([]byte, int, error) {
	endpoint := g.endpoint
	if method == http.MethodGet {
		endpoint += "?ref=" + Branch
	}
	req, e := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if e != nil {
		return nil, 0, errors.New("invalid checkpoint request")
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "ROMD-source-state")
	req.Header.Set("Cache-Control", "no-cache")
	response, e := g.client.Do(req)
	if e != nil {
		return nil, 0, errors.New("checkpoint request failed; outcome may be unknown")
	}
	defer response.Body.Close()
	b, e := io.ReadAll(io.LimitReader(response.Body, 2*MaxBytes+1))
	if e != nil || len(b) > 2*MaxBytes {
		return nil, response.StatusCode, errors.New("invalid checkpoint response size/body")
	}
	return b, response.StatusCode, nil
}
func (g *GitHub) Load(ctx context.Context) ([]byte, string, error) {
	b, status, e := g.request(ctx, http.MethodGet, nil)
	if e != nil {
		return nil, "", e
	}
	if status != 200 {
		return nil, "", fmt.Errorf("checkpoint read HTTP %d; no automatic initialization", status)
	}
	var file struct {
		Type, Encoding, Content, SHA string
		Size                         int
	}
	if e = json.Unmarshal(b, &file); e != nil {
		return nil, "", errors.New("invalid checkpoint file response")
	}
	if file.Type != "file" || file.Encoding != "base64" || file.Size < 1 || file.Size > MaxBytes || !revision.MatchString(file.SHA) {
		return nil, "", errors.New("unsupported checkpoint file")
	}
	raw, e := base64.StdEncoding.DecodeString(file.Content)
	if e != nil || len(raw) != file.Size || blobSHA(raw) != file.SHA {
		return nil, "", errors.New("checkpoint content integrity mismatch")
	}
	return raw, file.SHA, nil
}
func (g *GitHub) CompareAndSwap(ctx context.Context, version string, raw []byte) (string, error) {
	if len(raw) < 1 || len(raw) > MaxBytes || (version != "" && !revision.MatchString(version)) {
		return "", errors.New("invalid checkpoint size/version")
	}
	payload := map[string]string{"message": "chore(state): checkpoint Redump source scheduling", "branch": Branch, "content": base64.StdEncoding.EncodeToString(raw)}
	if version != "" {
		payload["sha"] = version
	}
	body, e := json.Marshal(payload)
	if e != nil {
		return "", e
	}
	b, status, e := g.request(ctx, http.MethodPut, body)
	if e != nil {
		return "", e
	}
	if status == 409 || status == 422 {
		return "", ErrConflict
	}
	if status != 200 && status != 201 {
		return "", errors.New("checkpoint write HTTP " + strconv.Itoa(status))
	}
	var receipt struct{ Content struct{ SHA string } }
	if e = json.Unmarshal(b, &receipt); e != nil || receipt.Content.SHA != blobSHA(raw) {
		return "", errors.New("checkpoint receipt invalid; outcome may be unknown")
	}
	return receipt.Content.SHA, nil
}
