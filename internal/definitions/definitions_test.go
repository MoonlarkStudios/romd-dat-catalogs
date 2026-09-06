package definitions

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	r, e := LoadSource("../../definitions")
	if e != nil {
		t.Fatal(e)
	}
	b, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestStrictDefinitions(t *testing.T) {
	good := fixture(t)
	r, e := Parse(good)
	if e != nil {
		t.Fatal(e)
	}
	if r.Catalogs["redump/psx/discs"].SystemID != "psx" {
		t.Fatal("lost system identity")
	}
	for name, b := range map[string][]byte{
		"duplicate top":       bytes.Replace(good, []byte(`"schemaVersion": 2`), []byte(`"schemaVersion": 2, "schemaVersion": 2`), 1),
		"duplicate escaped":   bytes.Replace(good, []byte(`"name": "PlayStation"`), []byte(`"name":"PlayStation", "\u006eame":"other"`), 1),
		"duplicate system":    bytes.Replace(good, []byte(`"psx": {`), []byte(`"psx": {}, "psx": {`), 1),
		"unknown":             bytes.Replace(good, []byte(`"name":`), []byte(`"extra":true,"name":`), 1),
		"case alias":          bytes.Replace(good, []byte(`"name":`), []byte(`"Name":`), 1),
		"missing":             bytes.Replace(good, []byte(`"provider": "redump",`), nil, 1),
		"unsupported version": bytes.Replace(good, []byte(`"schemaVersion": 2`), []byte(`"schemaVersion": 3`), 1),
		"fraction":            bytes.Replace(good, []byte(`10000`), []byte(`1.5`), 1),
		"null limits":         bytes.Replace(good, []byte(`10000`), []byte(`null`), 1),
		"unknown limit":       bytes.Replace(good, []byte(`"minimumGames"`), []byte(`"minGames"`), 1),
		"empty":               []byte(`{}`), "null": []byte(`null`), "array": []byte(`[]`),
		"trailing": append(append([]byte{}, good...), []byte(` {}`)...),
		"oversize": []byte(strings.Repeat(" ", MaxBytes+1)),
	} {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(b, good) {
				t.Fatal("fixture mutation did not apply")
			}
			if _, e := Parse(b); e == nil {
				t.Fatal("accepted bad definitions")
			}
		})
	}
}
func TestValidationAndCompatibility(t *testing.T) {
	mutations := map[string]func(*Registry){
		"unknown system": func(r *Registry) {
			c := r.Catalogs["redump/psx/discs"]
			c.SystemID = "missing"
			r.Catalogs["redump/psx/discs"] = c
		},
		"bad system ID": func(r *Registry) { r.Systems["../escape"] = System{Name: "Bad"} },
		"blank label":   func(r *Registry) { r.Systems["psx"] = System{Name: " "} },
		"unknown provider": func(r *Registry) {
			c := r.Catalogs["redump/psx/discs"]
			c.Provider = "arbitrary"
			r.Catalogs["redump/psx/discs"] = c
		},
		"unsupported representation": func(r *Registry) {
			c := r.Catalogs["redump/psx/discs"]
			c.Representation = "bios"
			r.Catalogs["redump/psx/discs"] = c
		},
		"provider path": func(r *Registry) {
			c := r.Catalogs["redump/psx/discs"]
			c.ProviderSystemID = "../psx"
			r.Catalogs["redump/psx/discs"] = c
		},
		"zero floor": func(r *Registry) {
			c := r.Catalogs["redump/psx/discs"]
			c.Validation.MinimumGames = 0
			r.Catalogs["redump/psx/discs"] = c
		},
		"negative floor": func(r *Registry) {
			c := r.Catalogs["redump/psx/discs"]
			c.Validation.MinimumROMs = -1
			r.Catalogs["redump/psx/discs"] = c
		},
		"wrong path": func(r *Registry) { r.Catalogs["redump/other/discs"] = r.Catalogs["redump/psx/discs"] },
		"conflicting upstream": func(r *Registry) {
			r.Systems["other"] = System{Name: "Other", ManufacturerIDs: []string{}, Aliases: []string{}, ProviderMappings: map[string]string{}}
			c := r.Catalogs["redump/psx/discs"]
			c.SystemID = "other"
			r.Catalogs["redump/other/discs"] = c
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			r, _ := Parse(fixture(t))
			mutate(r)
			b, _ := json.Marshal(r)
			if _, e := Parse(b); e == nil {
				t.Fatal("accepted invalid mapping")
			}
		})
	}
	old, _ := Parse(fixture(t))
	next, _ := Parse(fixture(t))
	c := next.Catalogs["redump/psx/discs"]
	c.ProviderSystemID = "different"
	next.Catalogs["redump/psx/discs"] = c
	if e := next.Compatible(old); e == nil {
		t.Fatal("silently repointed catalog")
	}
	c.ProviderSystemID = "psx"
	c.Validation.MinimumGames++
	next.Catalogs["redump/psx/discs"] = c
	system := next.Systems["psx"]
	system.Name = "PlayStation revised label"
	next.Systems["psx"] = system
	if e := next.Compatible(old); e != nil {
		t.Fatal("reviewed label/floor edits should be allowed", e)
	}
	delete(next.Catalogs, "redump/psx/discs")
	if e := next.Compatible(old); e == nil {
		t.Fatal("removed stable ID")
	}
}
func TestDeterministicSerialization(t *testing.T) {
	a, _ := Parse(fixture(t))
	a.Systems["other"] = System{Name: "Other", ManufacturerIDs: []string{}, Aliases: []string{}, ProviderMappings: map[string]string{}}
	b := &Registry{Regions: a.Regions, Languages: a.Languages, Companies: a.Companies, SchemaVersion: a.SchemaVersion, Systems: map[string]System{}, Catalogs: map[string]Catalog{}}
	b.Systems["other"] = a.Systems["other"]
	for id, system := range a.Systems {
		b.Systems[id] = system
	}
	for id, catalog := range a.Catalogs {
		b.Catalogs[id] = catalog
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	if !bytes.Equal(x, y) {
		t.Fatal("map insertion order changed output")
	}
}

func TestNoIntroRegistryIdentity(t *testing.T) {
	r, err := LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	c := r.Catalogs["no-intro/snes/standard"]
	if c.SystemID != "snes" || c.Provider != "no-intro" || c.ProviderSystemID != "49" || c.ExpectedName != "Nintendo - Super Nintendo Entertainment System" || c.Representation != "standard" {
		t.Fatal("wrong stable No-Intro identity", c)
	}
	for _, system := range []string{"0", "049", "snes", "49?token=x"} {
		bad := c
		bad.ProviderSystemID = system
		r.Catalogs["no-intro/snes/standard"] = bad
		if err := r.Validate(); err == nil {
			t.Fatal("invalid upstream identifier", system)
		}
	}
	r.Catalogs["no-intro/snes/standard"] = c
	prior, err := LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	c.ProviderSystemID = "24"
	r.Catalogs["no-intro/snes/standard"] = c
	if err := r.Compatible(prior); err == nil {
		t.Fatal("silent provider reassignment")
	}
}
