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
		"duplicate top":       bytes.Replace(good, []byte(`"schemaVersion": 1`), []byte(`"schemaVersion": 1, "schemaVersion": 1`), 1),
		"duplicate escaped":   bytes.Replace(good, []byte(`"name": "Sony PlayStation"`), []byte(`"name":"Sony PlayStation", "\u006eame":"other"`), 1),
		"duplicate system":    bytes.Replace(good, []byte(`"psx": {`), []byte(`"psx": {}, "psx": {`), 1),
		"unknown":             bytes.Replace(good, []byte(`"name":`), []byte(`"extra":true,"name":`), 1),
		"case alias":          bytes.Replace(good, []byte(`"name":`), []byte(`"Name":`), 1),
		"missing":             bytes.Replace(good, []byte(`"provider": "redump",`), nil, 1),
		"unsupported version": bytes.Replace(good, []byte(`"schemaVersion": 1`), []byte(`"schemaVersion": 2`), 1),
		"fraction":            bytes.Replace(good, []byte(`10000`), []byte(`1.5`), 1),
		"null limits":         bytes.Replace(good, []byte(`10000`), []byte(`null`), 1),
		"unknown limit":       bytes.Replace(good, []byte(`"minimumGames"`), []byte(`"minGames"`), 1),
		"empty":               []byte(`{}`), "null": []byte(`null`), "array": []byte(`[]`),
		"trailing": append(append([]byte{}, good...), []byte(` {}`)...),
		"oversize": []byte(strings.Repeat(" ", MaxBytes+1)),
	} {
		t.Run(name, func(t *testing.T) {
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
			r.Systems["other"] = System{Name: "Other", ManufacturerIDs: []string{}}
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
	next.Systems["psx"] = System{Name: "PlayStation", ManufacturerIDs: []string{"sony"}}
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
	a.Systems["other"] = System{Name: "Other", ManufacturerIDs: []string{}}
	b := &Registry{Companies: a.Companies, SchemaVersion: 1, Systems: map[string]System{}, Catalogs: map[string]Catalog{}}
	b.Systems["other"] = a.Systems["other"]
	b.Systems["psx"] = a.Systems["psx"]
	b.Catalogs["redump/psx/discs"] = a.Catalogs["redump/psx/discs"]
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	if !bytes.Equal(x, y) {
		t.Fatal("map insertion order changed output")
	}
}
