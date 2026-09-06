package publisher

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const Format = "romd-dat-catalog-experimental-1"
const Trust = "unsigned-development"

var catalogID = regexp.MustCompile(`^[a-z0-9]+([./-][a-z0-9]+)*$`)
var objectPath = regexp.MustCompile(`^objects/[0-9a-f]{64}\.(dat|json|xml)$`)

type Reference struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}
type Attempt struct {
	RetryAt      *string `json:"retryAt,omitempty"`
	CatalogID    string  `json:"catalogId"`
	ExpectedName string  `json:"expectedName"`
	SourceURL    string  `json:"sourceUrl"`
	Path         *string `json:"path,omitempty"`
	Failure      *string `json:"failure,omitempty"`
}
type Provenance struct {
	SourceURL  string `json:"sourceUrl"`
	AcquiredAt string `json:"acquiredAt"`
}
type Catalog struct {
	RetryAt             *string     `json:"retryAt,omitempty"`
	Name                string      `json:"name"`
	Artifact            *Reference  `json:"artifact"`
	LastSuccessfulCheck *string     `json:"lastSuccessfulCheck"`
	LastChanged         *string     `json:"lastChanged"`
	LastAttempt         string      `json:"lastAttempt"`
	Health              string      `json:"health"`
	Error               *string     `json:"error"`
	Counts              *Counts     `json:"counts,omitempty"`
	Provenance          *Provenance `json:"provenance,omitempty"`
}
type Event struct {
	ID          string    `json:"id"`
	CatalogID   string    `json:"catalogId"`
	Name        string    `json:"name"`
	Sequence    int64     `json:"sequence"`
	PublishedAt string    `json:"publishedAt"`
	Artifact    Reference `json:"artifact"`
}
type Snapshot struct {
	Format      string             `json:"format"`
	Sequence    int64              `json:"sequence"`
	PublishedAt string             `json:"publishedAt"`
	Catalogs    map[string]Catalog `json:"catalogs"`
	Events      []Event            `json:"events"`
	Feed        Reference          `json:"feed"`
}
type Current struct {
	Format   string    `json:"format"`
	Trust    string    `json:"trust"`
	Sequence int64     `json:"sequence"`
	Snapshot Reference `json:"snapshot"`
}
type Options struct {
	Now          time.Time
	FeedLimit    int
	BeforeCommit func() error
}

func Hash(b []byte) string         { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func encode(v any) ([]byte, error) { b, e := json.Marshal(v); return append(b, '\n'), e }
func atomicWrite(name string, b []byte) error {
	if e := os.MkdirAll(filepath.Dir(name), 0755); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(name), ".upload-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, e = f.Write(b); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), name)
}
func put(root string, b []byte, suffix string) (Reference, error) {
	r := Reference{"objects/" + Hash(b) + "." + suffix, Hash(b), len(b)}
	name := filepath.Join(root, r.Path)
	old, e := readFile(name, MaxDocument)
	if e == nil {
		if !bytes.Equal(old, b) {
			return r, errors.New("immutable object corruption")
		}
		return r, nil
	}
	if !errors.Is(e, os.ErrNotExist) {
		return r, e
	}
	return r, atomicWrite(name, b)
}
func VerifiedRead(root string, r Reference) ([]byte, error) {
	if !objectPath.MatchString(r.Path) {
		return nil, errors.New("invalid object path")
	}
	b, e := readFile(filepath.Join(root, r.Path), MaxDocument)
	if e != nil {
		return nil, e
	}
	if len(b) != r.Bytes || Hash(b) != r.SHA256 {
		return nil, errors.New("object integrity failure")
	}
	return b, nil
}
func verifySnapshot(root string, s Snapshot) error {
	if _, e := VerifiedRead(root, s.Feed); e != nil {
		return e
	}
	for _, c := range s.Catalogs {
		if c.Artifact != nil {
			if _, e := VerifiedRead(root, *c.Artifact); e != nil {
				return e
			}
		}
	}
	for _, ev := range s.Events {
		if _, e := VerifiedRead(root, ev.Artifact); e != nil {
			return e
		}
	}
	return nil
}

// LoadSnapshot checks integrity, not authenticity of a remote publisher.
func LoadSnapshot(root string) (Snapshot, error) {
	var c Current
	var s Snapshot
	b, e := readFile(filepath.Join(root, "current.json"), MaxMetadata)
	if e != nil {
		return s, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return s, e
	}
	if c.Format != Format || c.Trust != Trust {
		return s, errors.New("unsupported metadata")
	}
	b, e = VerifiedRead(root, c.Snapshot)
	if e != nil {
		return s, e
	}
	if len(b) > MaxMetadata {
		return s, errors.New("metadata size limit")
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return s, e
	}
	if s.Format != Format || s.Sequence != c.Sequence || s.Sequence < 1 || s.Catalogs == nil {
		return s, errors.New("invalid snapshot")
	}
	return s, verifySnapshot(root, s)
}
func CatchUp(s Snapshot, installed map[string]string, rejected map[string][]string) map[string]Reference {
	result := map[string]Reference{}
	for key, c := range s.Catalogs {
		if c.Artifact == nil || installed[key] == c.Artifact.SHA256 {
			continue
		}
		skip := false
		for _, hash := range rejected[key] {
			if hash == c.Artifact.SHA256 {
				skip = true
			}
		}
		if !skip {
			result[key] = *c.Artifact
		}
	}
	return result
}
func publicURL(value string, httpsOnly bool) bool {
	u, e := url.Parse(value)
	return e == nil && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && (u.Scheme == "https" || (!httpsOnly && u.Scheme == "http"))
}

// Permit only the public Datomatic selection query, never session/download
// tokens or arbitrary query parameters that could carry credentials.
var datomaticSource = regexp.MustCompile(`^https://datomatic\.no-intro\.org/index\.php\?page=download&op=dat&s=[1-9][0-9]{0,5}$`)

func publicSourceURL(value string) bool {
	return publicURL(value, false) || datomaticSource.MatchString(value)
}

func Publish(root string, attempts []Attempt, baseURL string, opt Options) (Snapshot, error) {
	var empty Snapshot
	if !publicURL(baseURL, true) {
		return empty, errors.New("base URL must be public HTTPS without credentials/query/fragment")
	}
	baseURL = strings.TrimRight(baseURL, "/") + "/"
	if opt.FeedLimit == 0 {
		opt.FeedLimit = 100
	}
	if opt.FeedLimit < 1 || opt.FeedLimit > 1000 {
		return empty, errors.New("feed limit must be 1..1000")
	}
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	stamp := opt.Now.UTC().Format(time.RFC3339Nano)
	seen := map[string]bool{}
	for _, a := range attempts {
		if !catalogID.MatchString(a.CatalogID) || seen[a.CatalogID] {
			return empty, errors.New("invalid or duplicate catalog ID")
		}
		if a.RetryAt != nil {
			if _, err := time.Parse(time.RFC3339Nano, *a.RetryAt); err != nil || a.Failure == nil {
				return empty, errors.New("invalid acquisition retry time")
			}
		}
		seen[a.CatalogID] = true
		if a.ExpectedName == "" || !publicSourceURL(a.SourceURL) || (a.Path == nil) == (a.Failure == nil) {
			return empty, errors.New("invalid registry attempt")
		}
	}
	if e := os.MkdirAll(root, 0755); e != nil {
		return empty, e
	}
	unlock, e := lockPublisher(root)
	if e != nil {
		return empty, e
	}
	defer unlock()
	old, e := LoadSnapshot(root)
	if e != nil {
		if _, stat := os.Stat(filepath.Join(root, "current.json")); !errors.Is(stat, os.ErrNotExist) {
			return empty, e
		}
		old = Snapshot{Catalogs: map[string]Catalog{}, Events: []Event{}}
	}
	if old.PublishedAt != "" {
		previous, e := time.Parse(time.RFC3339Nano, old.PublishedAt)
		if e != nil || opt.Now.Before(previous) {
			return empty, errors.New("invalid or regressed publication clock")
		}
	}
	s := Snapshot{Format: Format, Sequence: old.Sequence + 1, PublishedAt: stamp, Catalogs: old.Catalogs, Events: []Event{}}
	ordered := append([]Attempt(nil), attempts...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].CatalogID < ordered[j].CatalogID })
	for _, a := range ordered {
		c, exists := s.Catalogs[a.CatalogID]
		if !exists {
			c.Name = a.ExpectedName
		}
		if c.Name != a.ExpectedName {
			return empty, errors.New("registry identity changed; explicit migration required")
		}
		c.LastAttempt = stamp
		var raw []byte
		var counts Counts
		var candidateErr error
		if a.Failure != nil {
			candidateErr = CandidateError("acquisition_failed")
			if *a.Failure == "publication_paused" {
				candidateErr = CandidateError("publication_paused")
			}
		} else {
			raw, candidateErr = readFile(*a.Path, MaxInput)
			if candidateErr == nil {
				raw, counts, candidateErr = Document(raw, a.ExpectedName)
			}
		}
		if candidateErr != nil {
			code := "io_failure"
			var ce CandidateError
			if errors.As(candidateErr, &ce) {
				code = string(ce)
			}
			if a.RetryAt != nil {
				c.RetryAt = a.RetryAt
			}
			c.Health = "failed"
			c.Error = &code
		} else {
			artifact, e := put(root, raw, "dat")
			if e != nil {
				return empty, e
			}
			if c.Artifact == nil || *c.Artifact != artifact {
				s.Events = append(s.Events, Event{fmt.Sprintf("urn:romd:dat:%d:%s:%s", s.Sequence, a.CatalogID, artifact.SHA256), a.CatalogID, c.Name, s.Sequence, stamp, artifact})
				c.Artifact = &artifact
				c.LastChanged = &stamp
				c.Counts = &counts
				c.Provenance = &Provenance{a.SourceURL, stamp}
			}
			c.LastSuccessfulCheck = &stamp
			c.RetryAt = nil
			c.Health = "healthy"
			c.Error = nil
		}
		s.Catalogs[a.CatalogID] = c
	}
	s.Events = append(s.Events, old.Events...)
	if len(s.Events) > opt.FeedLimit {
		s.Events = s.Events[:opt.FeedLimit]
	}
	feed, e := feedBytes(s.Events, baseURL)
	if e != nil {
		return empty, e
	}
	s.Feed, e = put(root, feed, "xml")
	if e != nil {
		return empty, e
	}
	b, e := encode(s)
	if e != nil {
		return empty, e
	}
	if len(b) > MaxMetadata {
		return empty, errors.New("metadata size limit")
	}
	ref, e := put(root, b, "json")
	if e != nil {
		return empty, e
	}
	if _, e = VerifiedRead(root, ref); e != nil {
		return empty, e
	}
	if e = verifySnapshot(root, s); e != nil {
		return empty, e
	}
	if opt.BeforeCommit != nil {
		if e = opt.BeforeCommit(); e != nil {
			return empty, e
		}
	}
	b, e = encode(Current{Format, Trust, s.Sequence, ref})
	if e != nil {
		return empty, e
	}
	if e = atomicWrite(filepath.Join(root, "current.json"), b); e != nil {
		return empty, e
	}
	return s, nil
}

func ReadManifest(name string) ([]Attempt, error) {
	b, e := readFile(name, MaxMetadata)
	if e != nil {
		return nil, e
	}
	var a []Attempt
	if e = json.Unmarshal(b, &a); e != nil {
		return nil, e
	}
	if a == nil {
		return nil, errors.New("manifest must be an array")
	}
	for i := range a {
		if a[i].Path != nil && !filepath.IsAbs(*a[i].Path) {
			p := filepath.Join(filepath.Dir(name), *a[i].Path)
			a[i].Path = &p
		}
	}
	return a, nil
}
