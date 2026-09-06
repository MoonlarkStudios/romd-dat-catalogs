package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/definitions"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/distribution"
)

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("commands: init, rotate, stage, restore, verify-assets, candidate, reference-data, export-data, validate-definitions")
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
	case "validate-definitions":
		path := fs.String("definitions", "definitions", "reviewed definitions directory")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("unexpected argument")
		}
		_, err := definitions.LoadSource(*path)
		return err
	case "export-data":
		state := fs.String("state", "output", "local publisher state")
		out := fs.String("out", "", "data repository checkout")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		if *out == "" || fs.NArg() != 0 {
			return errors.New("--out data checkout required")
		}
		return distribution.ExportData(*state, *out)
	case "stage":
		state := fs.String("state", "output", "local publisher state")
		trust := fs.String("trust", "trust", "public roots directory")
		keys := fs.String("keys", "", "online signing keys file")
		out := fs.String("out", "staged", "new staging directory")
		release := fs.String("release-base", "", "immutable release asset base URL")
		registryPath := fs.String("definitions", "", "reviewed definitions directory (Git publication)")
		repository := fs.String("data-repository", "", "GitHub owner/repo containing complete DATs")
		commit := fs.String("data-commit", "", "full pushed data commit SHA")
		version := fs.Int64("version", 0, "monotonic metadata version")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		k, e := distribution.ReadKeys(*keys)
		if e != nil {
			return e
		}
		opt := distribution.StageOptions{State: *state, TrustDir: *trust, Output: *out, ReleaseBase: *release, Keys: k, Version: *version, Now: time.Now().UTC()}
		if *registryPath != "" {
			if *repository == "" || *release != "" {
				return errors.New("definitions require Git publication")
			}
			opt.Definitions, e = definitions.LoadSource(*registryPath)
			if e != nil {
				return e
			}
		}
		var index distribution.Index
		if *repository != "" || *commit != "" {
			if *release != "" {
				return errors.New("Git publication and legacy release mode are exclusive")
			}
			index, e = distribution.StageGit(opt, *repository, *commit)
		} else {
			index, e = distribution.Stage(opt)
		}
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
		bootstrap := fs.Bool("bootstrap", false, "new independent site only; require destination HTTP 404")
		migration := fs.String("migration-site", "", "old signed site, only while destination does not exist")
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
		origin, e := distribution.RestoreOrigin(*site, *migration, *bootstrap, nil)
		if e != nil {
			return e
		}
		if origin == "" {
			return nil
		} // Explicit first deployment; publisher initializes local state.
		index, e := distribution.RefreshIndex(b, strings.TrimRight(origin, "/"), *cache, nil)
		if e != nil {
			return e
		}
		if *expected != 0 && index.Version != *expected {
			return errors.New("deployed publication version does not match expected version")
		}
		return distribution.RestorePublication(index, *state, nil)
	case "candidate", "reference-data":
		root := fs.String("root", "trust/1.root.json", "independently pinned public root")
		site := fs.String("site", "", "HTTPS publisher site")
		bundle := fs.String("bundle", "", "local signed Stage bundle; exclusive with --site")
		cache := fs.String("cache", "", "persistent cache dedicated to this publisher/trust root")
		id := fs.String("catalog", "", "exact stable catalog ID")
		name := fs.String("name", "", "expected DAT header name")
		out := fs.String("out", "", "new candidate directory")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		if fs.NArg() != 0 || *cache == "" || *out == "" || (*site == "") == (*bundle == "") {
			return errors.New("reader requires root, cache, out, and exactly one of site/bundle")
		}
		if args[0] == "candidate" && (*id == "" || *name == "") {
			return errors.New("candidate requires catalog and name")
		}
		if args[0] == "reference-data" && (*id != "" || *name != "") {
			return errors.New("reference-data does not select a DAT")
		}
		var client *http.Client
		if *bundle != "" {
			client = distribution.BundleClient(*bundle)
			*site = distribution.BundleSite
		} else if !strings.HasPrefix(*site, "https://") {
			return errors.New("HTTPS site required")
		}
		rootBytes, e := os.ReadFile(*root)
		if e != nil {
			return e
		}
		if e = os.MkdirAll(*cache, 0700); e != nil {
			return e
		}
		gate, e := os.OpenFile(filepath.Join(*cache, "candidate.lock"), os.O_CREATE|os.O_RDWR, 0600)
		if e != nil {
			return e
		}
		defer gate.Close()
		if e = syscall.Flock(int(gate.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
			return errors.New("candidate cache is in use; retry after the active reader finishes")
		}
		defer syscall.Flock(int(gate.Fd()), syscall.LOCK_UN)
		index, e := distribution.RefreshIndex(rootBytes, strings.TrimRight(*site, "/"), *cache, client)
		if e != nil {
			return e
		}
		if args[0] == "reference-data" {
			if index.Definitions == nil {
				return errors.New("publisher has no shared reference data")
			}
			return writeReferenceData(*out, index)
		}
		candidate, document, e := distribution.ReadCandidate(index, *id, *name, client)
		if e != nil {
			return e
		}
		if e = os.Mkdir(*out, 0700); e != nil {
			return e
		}
		complete := false
		defer func() {
			if !complete {
				os.RemoveAll(*out)
			}
		}()
		if e = os.WriteFile(filepath.Join(*out, "candidate.dat"), document, 0600); e != nil {
			return e
		}
		info, e := json.Marshal(candidate)
		if e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(*out, "candidate.json"), info, 0600); e != nil {
			return e
		}
		complete = true
		return nil
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
		return distribution.VerifyDownloads(index, nil)
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

// The authenticated index is authoritative for both definitions and available
// catalogs. Never fetch the unauthenticated Pages alias or download a DAT here.
func writeReferenceData(out string, index distribution.Index) error {
	reference, err := json.Marshal(index.Definitions)
	if err != nil {
		return err
	}
	publication, err := json.Marshal(index)
	if err != nil {
		return err
	}
	if err := os.Mkdir(out, 0700); err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			os.RemoveAll(out)
		}
	}()
	if err := os.WriteFile(filepath.Join(out, "reference-data.json"), reference, 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "catalog.json"), publication, 0600); err != nil {
		return err
	}
	complete = true
	return nil
}
