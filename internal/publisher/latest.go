package publisher

import (
	"errors"
	"os"
	"path/filepath"
)

// Latest keeps at most one content-change event per catalog. Its stable GUID
// survives unchanged checks; historical documents are not needed to restore it.
func Latest(s Snapshot) (Snapshot, []byte, error) {
	events := []Event{}
	seen := map[string]bool{}
	for _, event := range s.Events {
		c, ok := s.Catalogs[event.CatalogID]
		if ok && c.Artifact != nil && *c.Artifact == event.Artifact && !seen[event.CatalogID] {
			events = append(events, event)
			seen[event.CatalogID] = true
		}
	}
	s.Events = events
	feed, err := feedBytes(events, "https://catalogs.example.invalid/")
	if err != nil {
		return s, nil, err
	}
	s.Feed = Reference{"objects/" + Hash(feed) + ".xml", Hash(feed), len(feed)}
	return s, feed, nil
}

// RestoreLatest rebuilds disposable local state from authenticated latest-only
// metadata and hash-verified documents. It never replaces an existing directory.
func RestoreLatest(destination string, s Snapshot, documents map[string][]byte) error {
	if s.Format != Format || s.Sequence < 1 || s.Catalogs == nil {
		return errors.New("invalid latest snapshot")
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		return errors.New("restore destination must not exist")
	}
	tmp, err := os.MkdirTemp(filepath.Dir(destination), ".restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, c := range s.Catalogs {
		if c.Artifact == nil {
			continue
		}
		b, ok := documents[c.Artifact.Path]
		if !ok || Hash(b) != c.Artifact.SHA256 || len(b) != c.Artifact.Bytes {
			return errors.New("document binding mismatch")
		}
		doc, _, err := Document(b, c.Name)
		if err != nil || Hash(doc) != Hash(b) {
			return errors.New("invalid extracted document")
		}
		ref, err := put(tmp, b, "dat")
		if err != nil {
			return err
		}
		if ref != *c.Artifact {
			return errors.New("invalid document reference")
		}
	}
	feed, err := feedBytes(s.Events, "https://catalogs.example.invalid/")
	if err != nil {
		return err
	}
	ref, err := put(tmp, feed, "xml")
	if err != nil {
		return err
	}
	if ref != s.Feed {
		return errors.New("feed binding mismatch")
	}
	if err := verifySnapshot(tmp, s); err != nil {
		return err
	}
	b, err := encode(s)
	if err != nil {
		return err
	}
	ref, err = put(tmp, b, "json")
	if err != nil {
		return err
	}
	b, err = encode(Current{Format, Trust, s.Sequence, ref})
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(tmp, "current.json"), b); err != nil {
		return err
	}
	return os.Rename(tmp, destination)
}
