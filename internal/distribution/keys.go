package distribution

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

type Keys map[string][]byte

var roles = []string{"root", "targets", "snapshot", "timestamp"}

func signer(keys Keys, role string) (signature.Signer, error) {
	key := keys[role]
	if len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("missing or invalid %s signing key", role)
	}
	return signature.LoadSigner(ed25519.PrivateKey(key), crypto.Hash(0))
}
func ReadKeys(name string) (Keys, error) {
	b, e := os.ReadFile(name)
	if e != nil {
		return nil, e
	}
	var keys Keys
	e = json.Unmarshal(b, &keys)
	return keys, e
}

// Initialize creates a separate key directory and public root chain. The root key
// stays on the operator's machine; only online.json belongs in the CI secret.
func Initialize(dir string, now time.Time, previousRoot []byte, oldKeys Keys) error {
	if _, e := os.Stat(dir); !errors.Is(e, os.ErrNotExist) {
		return errors.New("initialization directory must not exist")
	}
	root := metadata.Root(now.AddDate(1, 0, 0))
	keys := Keys{}
	online := Keys{}
	for _, role := range roles {
		_, key, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return e
		}
		keys[role] = key
		public, e := metadata.KeyFromPublicKey(key.Public())
		if e != nil {
			return e
		}
		if e = root.Signed.AddKey(public, role); e != nil {
			return e
		}
		if role != "root" {
			online[role] = key
		}
	}
	if len(previousRoot) > 0 {
		old, e := metadata.Root().FromBytes(previousRoot)
		if e != nil {
			return e
		}
		if e = old.VerifyDelegate("root", old); e != nil {
			return e
		}
		root.Signed.Version = old.Signed.Version + 1
		s, e := signer(oldKeys, "root")
		if e != nil {
			return e
		}
		if _, e = root.Sign(s); e != nil {
			return e
		}
	}
	s, e := signer(keys, "root")
	if e != nil {
		return e
	}
	if _, e = root.Sign(s); e != nil {
		return e
	}
	if e = root.VerifyDelegate("root", root); e != nil {
		return e
	}
	if len(previousRoot) > 0 {
		old, _ := metadata.Root().FromBytes(previousRoot)
		if e = old.VerifyDelegate("root", root); e != nil {
			return e
		}
	}
	if e = os.MkdirAll(filepath.Join(dir, "public"), 0700); e != nil {
		return e
	}
	b, e := root.ToBytes(false)
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(dir, "public", fmt.Sprintf("%d.root.json", root.Signed.Version)), b, 0644); e != nil {
		return e
	}
	for name, value := range map[string]Keys{"offline-root.json": {"root": keys["root"]}, "online.json": online} {
		b, e := json.Marshal(value)
		if e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
			return e
		}
	}
	return nil
}

func readRoot(dir string) (*metadata.Metadata[metadata.RootType], error) {
	entries, e := os.ReadDir(dir)
	if e != nil {
		return nil, e
	}
	var root *metadata.Metadata[metadata.RootType]
	for _, entry := range entries {
		var version int
		var rest string
		if _, e := fmt.Sscanf(entry.Name(), "%d.root.%s", &version, &rest); e != nil || rest != "json" {
			continue
		}
		r, e := metadata.Root().FromFile(filepath.Join(dir, entry.Name()))
		if e != nil {
			return nil, e
		}
		if r.Signed.Version != int64(version) {
			return nil, errors.New("root filename/version mismatch")
		}
		if root == nil || r.Signed.Version > root.Signed.Version {
			root = r
		}
	}
	if root == nil {
		return nil, errors.New("missing versioned root")
	}
	if e = root.VerifyDelegate("root", root); e != nil {
		return nil, e
	}
	return root, nil
}
