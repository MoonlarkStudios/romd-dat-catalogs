package publisher

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)

const base = "https://catalogs.example.invalid/"

func ptr(s string) *string { return &s }
func dat(v int) []byte {
	return []byte(fmt.Sprintf(`<datafile><header><name>Synthetic Console</name><version>%d</version></header><game name="Game %d"><rom name="rom.bin" size="1" crc="d202ef8d"/></game></datafile>`, v, v))
}
func write(t *testing.T, p string, b []byte) {
	t.Helper()
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
}

type fixture struct {
	root, input string
	a           Attempt
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "input.dat")
	write(t, input, dat(1))
	return fixture{filepath.Join(dir, "output"), input, Attempt{CatalogID: "synthetic/console/standard", ExpectedName: "Synthetic Console", SourceURL: "https://example.invalid/dat", Path: &input}}
}
func mustPublish(t *testing.T, f fixture, attempts ...Attempt) Snapshot {
	t.Helper()
	if attempts == nil {
		attempts = []Attempt{f.a}
	}
	s, e := Publish(f.root, attempts, base, Options{Now: now})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func zipData(t *testing.T, raw []byte, names ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, name := range names {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0644)
		f, e := w.CreateHeader(h)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.Write(raw); e != nil {
			t.Fatal(e)
		}
	}
	if e := w.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestFirstSubscription(t *testing.T) {
	f := newFixture(t)
	s := mustPublish(t, f)
	loaded, e := LoadSnapshot(f.root)
	if e != nil || !reflect.DeepEqual(s, loaded) {
		t.Fatalf("load: %v", e)
	}
	c := CatchUp(loaded, nil, nil)
	b, e := VerifiedRead(f.root, c[f.a.CatalogID])
	if e != nil || !bytes.Equal(b, dat(1)) {
		t.Fatalf("bytes: %v", e)
	}
}
func TestRepackedZIP(t *testing.T) {
	f := newFixture(t)
	write(t, f.input, zipData(t, dat(1), "one.dat"))
	first := mustPublish(t, f)
	write(t, f.input, zipData(t, dat(1), "renamed.dat"))
	second := mustPublish(t, f)
	if !reflect.DeepEqual(first.Events, second.Events) {
		t.Fatal("repack created event")
	}
}
func TestFeedTruncationCatchUp(t *testing.T) {
	f := newFixture(t)
	mustPublish(t, f)
	var s Snapshot
	for v := 2; v <= 4; v++ {
		write(t, f.input, dat(v))
		var e error
		s, e = Publish(f.root, []Attempt{f.a}, base, Options{Now: now, FeedLimit: 1})
		if e != nil {
			t.Fatal(e)
		}
	}
	if len(s.Events) != 1 {
		t.Fatal("retention")
	}
	key := f.a.CatalogID
	c := CatchUp(s, map[string]string{key: Hash(dat(1))}, nil)
	if c[key].SHA256 != Hash(dat(4)) {
		t.Fatal("did not catch up")
	}
	if len(CatchUp(s, map[string]string{key: Hash(dat(4))}, nil)) != 0 {
		t.Fatal("duplicate candidate")
	}
}
func TestRejectedVersion(t *testing.T) {
	f := newFixture(t)
	s := mustPublish(t, f)
	if len(CatchUp(s, nil, map[string][]string{f.a.CatalogID: {Hash(dat(1))}})) != 0 {
		t.Fatal("reapplied rejection")
	}
}
func TestRestoreOldDocument(t *testing.T) {
	f := newFixture(t)
	first := mustPublish(t, f)
	write(t, f.input, dat(2))
	mustPublish(t, f)
	write(t, f.input, dat(1))
	last := mustPublish(t, f)
	if first.Events[0].Artifact != last.Events[0].Artifact || first.Events[0].ID == last.Events[0].ID {
		t.Fatal("restoration identity")
	}
}
func TestPartialFailure(t *testing.T) {
	f := newFixture(t)
	first := mustPublish(t, f)
	fail := f.a
	fail.Path = nil
	fail.Failure = ptr("private diagnostics")
	other := f.a
	other.CatalogID = "synthetic/second/standard"
	second, e := Publish(f.root, []Attempt{fail, other}, base, Options{Now: now.Add(time.Hour)})
	if e != nil {
		t.Fatal(e)
	}
	c := second.Catalogs[f.a.CatalogID]
	before := first.Catalogs[f.a.CatalogID]
	if *c.Artifact != *before.Artifact || *c.LastSuccessfulCheck != *before.LastSuccessfulCheck || *c.Error != "acquisition_failed" || second.Catalogs[other.CatalogID].Health != "healthy" {
		t.Fatal("failure isolation")
	}
}
func TestFirstFailure(t *testing.T) {
	f := newFixture(t)
	os.Remove(f.input)
	s := mustPublish(t, f)
	c := s.Catalogs[f.a.CatalogID]
	if c.Artifact != nil || c.LastSuccessfulCheck != nil || len(CatchUp(s, nil, nil)) != 0 {
		t.Fatal("first failure")
	}
}
func TestMissingSweepRetainsCatalog(t *testing.T) {
	f := newFixture(t)
	first := mustPublish(t, f)
	second, e := Publish(f.root, []Attempt{}, base, Options{Now: now})
	if e != nil || !reflect.DeepEqual(first.Catalogs, second.Catalogs) {
		t.Fatalf("missing sweep: %v", e)
	}
}
func TestVariantMismatch(t *testing.T) {
	f := newFixture(t)
	first := mustPublish(t, f)
	write(t, f.input, bytes.ReplaceAll(dat(1), []byte("Synthetic Console"), []byte("Other Variant")))
	second := mustPublish(t, f)
	if *second.Catalogs[f.a.CatalogID].Error != "identity_mismatch" || !reflect.DeepEqual(first.Events, second.Events) {
		t.Fatal("identity bypass")
	}
}
func TestInterruptedCommit(t *testing.T) {
	f := newFixture(t)
	first := mustPublish(t, f)
	old, _ := os.ReadFile(filepath.Join(f.root, "current.json"))
	write(t, f.input, dat(2))
	_, e := Publish(f.root, []Attempt{f.a}, base, Options{Now: now, BeforeCommit: func() error { return errors.New("injected") }})
	if e == nil {
		t.Fatal("no interruption")
	}
	current, _ := os.ReadFile(filepath.Join(f.root, "current.json"))
	loaded, e := LoadSnapshot(f.root)
	if !bytes.Equal(old, current) || e != nil || !reflect.DeepEqual(first, loaded) {
		t.Fatal("old state changed")
	}
	if mustPublish(t, f).Sequence != 2 {
		t.Fatal("retry")
	}
}
func TestTampering(t *testing.T) {
	for _, historical := range []bool{false, true} {
		t.Run(fmt.Sprint(historical), func(t *testing.T) {
			f := newFixture(t)
			first := mustPublish(t, f)
			if historical {
				write(t, f.input, dat(2))
				mustPublish(t, f)
			}
			write(t, filepath.Join(f.root, first.Events[0].Artifact.Path), []byte("corrupt"))
			if _, e := LoadSnapshot(f.root); e == nil {
				t.Fatal("load accepted corruption")
			}
			if _, e := Publish(f.root, []Attempt{f.a}, base, Options{Now: now}); e == nil {
				t.Fatal("publish accepted corruption")
			}
		})
	}
}
func TestRSS(t *testing.T) {
	f := newFixture(t)
	s := mustPublish(t, f)
	b, e := VerifiedRead(f.root, s.Feed)
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Items []struct {
			GUID      string `xml:"guid"`
			Enclosure struct {
				URL    string `xml:"url,attr"`
				Length int    `xml:"length,attr"`
			} `xml:"enclosure"`
			Hash      string `xml:"urn:romd:dat-catalog:experimental:1 sha256"`
			CatalogID string `xml:"urn:romd:dat-catalog:experimental:1 catalogId"`
		} `xml:"channel>item"`
	}
	if e = xml.Unmarshal(b, &r); e != nil {
		t.Fatal(e)
	}
	if len(r.Items) != 1 {
		t.Fatal("missing event")
	}
	item := r.Items[0]
	if item.GUID != s.Events[0].ID || item.Hash != Hash(dat(1)) || item.Enclosure.Length != len(dat(1)) || item.Enclosure.URL != base+s.Events[0].Artifact.Path || item.CatalogID != f.a.CatalogID {
		t.Fatalf("RSS mismatch: %+v", item)
	}
}
func TestDeterministicOrdering(t *testing.T) {
	f := newFixture(t)
	other := f.a
	other.CatalogID = "synthetic/another/standard"
	first := mustPublish(t, f, f.a, other)
	f.root = filepath.Join(t.TempDir(), "output")
	second := mustPublish(t, f, other, f.a)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("nondeterministic")
	}
}
func TestProcessHelper(t *testing.T) {
	if os.Getenv("ROMD_PUBLISH_HELPER") != "1" {
		return
	}
	a := Attempt{CatalogID: os.Getenv("ROMD_CATALOG_ID"), ExpectedName: "Synthetic Console", SourceURL: "https://example.invalid/dat", Path: ptr(os.Getenv("ROMD_INPUT"))}
	if _, e := Publish(os.Getenv("ROMD_OUTPUT"), []Attempt{a}, base, Options{Now: now}); e != nil {
		t.Fatal(e)
	}
}
func TestConcurrentProcesses(t *testing.T) {
	f := newFixture(t)
	cmds := []*exec.Cmd{}
	for i := 0; i < 3; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestProcessHelper$")
		cmd.Env = append(os.Environ(), "ROMD_PUBLISH_HELPER=1", "ROMD_CATALOG_ID=synthetic/console"+fmt.Sprint(i)+"/standard", "ROMD_INPUT="+f.input, "ROMD_OUTPUT="+f.root)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if e := cmd.Start(); e != nil {
			t.Fatal(e)
		}
		cmds = append(cmds, cmd)
		t.Cleanup(func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		})
	}
	for _, cmd := range cmds {
		if e := cmd.Wait(); e != nil {
			t.Fatal(e)
		}
	}
	s, e := LoadSnapshot(f.root)
	if e != nil || s.Sequence != 3 || len(s.Catalogs) != 3 {
		t.Fatalf("lost publisher: %v", e)
	}
}
func TestBadArchives(t *testing.T) {
	for _, name := range []string{"../catalog.dat", "/catalog.dat", "nested\\catalog.dat", "catalog.exe"} {
		t.Run(name, func(t *testing.T) {
			if _, _, e := Document(zipData(t, dat(1), name), "Synthetic Console"); e == nil {
				t.Fatal("unsafe archive")
			}
		})
	}
	if _, _, e := Document(zipData(t, dat(1), "a.dat", "b.dat"), "Synthetic Console"); e == nil {
		t.Fatal("ambiguous archive")
	}
}
func TestBadXML(t *testing.T) {
	inputs := []string{`<html>error</html>`, `<broken`, `<datafile><header><name>Synthetic Console</name></header></datafile>`, string(dat(1)) + string(dat(1)), strings.Replace(string(dat(1)), "</datafile>", `<game name="Game 1"/></datafile>`, 1), `<!DOCTYPE datafile [<!ENTITY x SYSTEM "file:///etc/passwd">]>` + string(dat(1)), `<!DOCTYPE datafile [<!ENTITY x "harmless">]>` + string(dat(1)), "garbage" + string(dat(1))}
	for i, input := range inputs {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, _, e := Document([]byte(input), "Synthetic Console"); e == nil {
				t.Fatal("accepted invalid XML")
			}
		})
	}
}
func TestDoctypeNoNetwork(t *testing.T) {
	b, e := os.ReadFile("../../fixtures/example.dat")
	if e != nil {
		t.Fatal(e)
	}
	out, c, e := Document(b, "ROMD Synthetic Console")
	if e != nil || !bytes.Equal(out, b) || c.Games != 1 {
		t.Fatalf("doctype: %v", e)
	}
}

func TestUTF8BOMPreserved(t *testing.T) {
	raw := append([]byte{0xef, 0xbb, 0xbf}, dat(1)...)
	out, counts, err := Document(raw, "Synthetic Console")
	if err != nil || !bytes.Equal(out, raw) || counts.Games != 1 {
		t.Fatalf("BOM document: %v", err)
	}
}

func TestDuplicateXMLAttributesRejected(t *testing.T) {
	raw := bytes.Replace(dat(1), []byte(`name="Game 1"`), []byte(`name="Game 1" name="Game 2"`), 1)
	if _, _, err := Document(raw, "Synthetic Console"); err == nil {
		t.Fatal("duplicate XML attributes accepted")
	}
}
func TestSizeLimits(t *testing.T) {
	if _, _, e := Document(make([]byte, MaxInput+1), "x"); e == nil {
		t.Fatal("input limit")
	}
	large := bytes.Repeat([]byte("a"), MaxDocument+1)
	if _, _, e := Document(zipData(t, large, "catalog.dat"), "x"); e == nil {
		t.Fatal("decompression limit")
	}
	if _, e := readBounded(strings.NewReader("123"), 2); e == nil {
		t.Fatal("read limit")
	}
}
func TestRegistryValidation(t *testing.T) {
	f := newFixture(t)
	if _, e := Publish(f.root, []Attempt{f.a, f.a}, base, Options{}); e == nil {
		t.Fatal("duplicate IDs")
	}
	mustPublish(t, f)
	a := f.a
	a.ExpectedName = "renamed"
	if _, e := Publish(f.root, []Attempt{a}, base, Options{Now: now}); e == nil {
		t.Fatal("rename")
	}
	for _, url := range []string{"https://user:secret@example.invalid/", "https://example.invalid/?token=secret", "file:///private/data"} {
		a = f.a
		a.SourceURL = url
		if _, e := Publish(f.root, []Attempt{a}, base, Options{}); e == nil {
			t.Fatal("unsafe provenance")
		}
	}
}
func TestClockRegression(t *testing.T) {
	f := newFixture(t)
	mustPublish(t, f)
	if _, e := Publish(f.root, []Attempt{f.a}, base, Options{Now: now.Add(-time.Hour)}); e == nil {
		t.Fatal("clock regression")
	}
}
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	e := filepath.WalkDir(src, func(name string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(src, name)
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		b, e := os.ReadFile(name)
		if e != nil {
			return e
		}
		return os.WriteFile(target, b, 0600)
	})
	if e != nil {
		t.Fatal(e)
	}
}
func TestResumePythonSnapshot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "output")
	copyTree(t, "../../fixtures/python-snapshot", root)
	before, e := LoadSnapshot(root)
	if e != nil {
		t.Fatal(e)
	}
	a := Attempt{CatalogID: "synthetic/console/standard", ExpectedName: "ROMD Synthetic Console", SourceURL: "https://example.invalid/synthetic-dat", Path: ptr("../../fixtures/example.dat")}
	at, e := time.Parse(time.RFC3339Nano, before.PublishedAt)
	if e != nil {
		t.Fatal(e)
	}
	after, e := Publish(root, []Attempt{a}, base, Options{Now: at.Add(time.Second)})
	if e != nil {
		t.Fatal(e)
	}
	if after.Sequence != before.Sequence+1 || !reflect.DeepEqual(before.Events, after.Events) || *before.Catalogs[a.CatalogID].Artifact != *after.Catalogs[a.CatalogID].Artifact {
		t.Fatal("Python resume changed DAT identity or events")
	}
}
func TestMetadataTamper(t *testing.T) {
	f := newFixture(t)
	mustPublish(t, f)
	b, e := os.ReadFile(filepath.Join(f.root, "current.json"))
	if e != nil {
		t.Fatal(e)
	}
	var c Current
	json.Unmarshal(b, &c)
	c.Trust = "signed"
	b, _ = encode(c)
	write(t, filepath.Join(f.root, "current.json"), b)
	if _, e = LoadSnapshot(f.root); e == nil {
		t.Fatal("unsupported trust")
	}
}
func BenchmarkDocument(b *testing.B) {
	var data bytes.Buffer
	data.WriteString(`<datafile><header><name>Synthetic Console</name></header>`)
	for i := 0; i < 10000; i++ {
		fmt.Fprintf(&data, `<game name="Game %d"><rom name="rom" size="1" crc="d202ef8d"/></game>`, i)
	}
	data.WriteString(`</datafile>`)
	raw := data.Bytes()
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, e := Document(raw, "Synthetic Console"); e != nil {
			b.Fatal(e)
		}
	}
}
func FuzzDocument(f *testing.F) {
	f.Add(dat(1))
	f.Add([]byte("<broken"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		_, _, _ = Document(b, "Synthetic Console")
	})
}

func TestRetryGuidancePreservesArtifactAndClearsOnSuccess(t *testing.T) {
	f := newFixture(t)
	first := mustPublish(t, f)
	failed := f.a
	failed.Path = nil
	failed.Failure = ptr("rate_limited")
	failed.RetryAt = ptr(now.Add(72 * time.Hour).UTC().Format(time.RFC3339Nano))
	second, e := Publish(f.root, []Attempt{failed}, base, Options{Now: now.Add(time.Hour)})
	if e != nil {
		t.Fatal(e)
	}
	c := second.Catalogs[f.a.CatalogID]
	if c.RetryAt == nil || *c.RetryAt != *failed.RetryAt || *c.Artifact != *first.Catalogs[f.a.CatalogID].Artifact || len(second.Events) != len(first.Events) {
		t.Fatal("failure changed content or lost retry guidance")
	}
	final, e := Publish(f.root, []Attempt{f.a}, base, Options{Now: now.Add(73 * time.Hour)})
	if e != nil {
		t.Fatal(e)
	}
	if final.Catalogs[f.a.CatalogID].RetryAt != nil || final.Catalogs[f.a.CatalogID].Health != "healthy" || len(final.Events) != len(first.Events) {
		t.Fatal("successful identical document must clear backoff without an update event")
	}
}
