package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JackSkylark/romd-dat-catalogs/internal/distribution"
)

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("commands: init, rotate, stage, restore, verify-assets")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	switch args[0] {
	case "init", "rotate":
		out := fs.String("out", "", "new key directory")
		previous := fs.String("previous-root", "", "previous public root (rotation only)")
		keys := fs.String("offline-key", "", "old offline root key file")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		if *out == "" {
			return errors.New("--out required")
		}
		var b []byte
		var k distribution.Keys
		var e error
		if args[0] == "rotate" {
			b, e = os.ReadFile(*previous)
			if e != nil {
				return e
			}
			k, e = distribution.ReadKeys(*keys)
			if e != nil {
				return e
			}
		}
		return distribution.Initialize(*out, time.Now().UTC(), b, k)
	case "stage":
		state := fs.String("state", "output", "local publisher state")
		trust := fs.String("trust", "trust", "public roots directory")
		keys := fs.String("keys", "", "online signing keys file")
		out := fs.String("out", "staged", "new staging directory")
		release := fs.String("release-base", "", "immutable release asset base URL")
		version := fs.Int64("version", 0, "monotonic metadata version")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		k, e := distribution.ReadKeys(*keys)
		if e != nil {
			return e
		}
		index, e := distribution.Stage(distribution.StageOptions{State: *state, TrustDir: *trust, Output: *out, ReleaseBase: *release, Keys: k, Version: *version, Now: time.Now().UTC()})
		if e != nil {
			return e
		}
		b, e := json.Marshal(index)
		if e != nil {
			return e
		}
		return os.WriteFile(filepath.Join(*out, "index.json"), b, 0644)
	case "restore":
		root := fs.String("root", "trust/1.root.json", "pinned bootstrap root")
		site := fs.String("site", "", "HTTPS Pages origin/path")
		cache := fs.String("cache", ".client-cache", "persistent TUF client cache")
		state := fs.String("state", "output", "new publisher state directory")
		expected := fs.Int64("expected-version", 0, "require an exact deployed metadata version")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		if !strings.HasPrefix(*site, "https://") {
			return errors.New("HTTPS site required")
		}
		b, e := os.ReadFile(*root)
		if e != nil {
			return e
		}
		index, e := distribution.RefreshIndex(b, strings.TrimRight(*site, "/"), *cache, nil)
		if e != nil {
			return e
		}
		if *expected != 0 && index.Version != *expected {
			return errors.New("deployed publication version does not match expected version")
		}
		raw, e := distribution.FetchAsset(index.State, nil)
		if e != nil {
			return e
		}
		return distribution.RestoreState(index, raw, *state)
	case "verify-assets":
		indexPath := fs.String("index", "staged/index.json", "locally staged index")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		b, e := os.ReadFile(*indexPath)
		if e != nil {
			return e
		}
		var index distribution.Index
		if e = json.Unmarshal(b, &index); e != nil {
			return e
		}
		for _, asset := range index.Downloads {
			if _, e = distribution.FetchAsset(asset, nil); e != nil {
				return e
			}
		}
		_, e = distribution.FetchAsset(index.State, nil)
		return e
	default:
		return errors.New("unknown distribution command")
	}
}
func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
