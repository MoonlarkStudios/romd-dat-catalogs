package definitions

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
)

func TestCompanyReferencesAndAliases(t *testing.T) {
	good := fixture(t)
	r, err := Parse(good)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Companies) != 15 || !slices.Equal(r.Systems["psx"].ManufacturerIDs, []string{"sony"}) {
		t.Fatal("seed manufacturer mapping missing")
	}
	cases := map[string]func(*Registry){
		"unknown manufacturer": func(r *Registry) {
			s := r.Systems["psx"]
			s.ManufacturerIDs = []string{"missing"}
			r.Systems["psx"] = s
		},
		"duplicate manufacturer": func(r *Registry) {
			s := r.Systems["psx"]
			s.ManufacturerIDs = []string{"sony", "sony"}
			r.Systems["psx"] = s
		},
		"null manufacturers":      func(r *Registry) { s := r.Systems["psx"]; s.ManufacturerIDs = nil; r.Systems["psx"] = s },
		"invalid company ID":      func(r *Registry) { r.Companies["../sony"] = Company{Name: "Other", Aliases: []string{}} },
		"blank name":              func(r *Registry) { r.Companies["sony"] = Company{Name: " ", Aliases: []string{}} },
		"null aliases":            func(r *Registry) { r.Companies["sony"] = Company{Name: "Sony"} },
		"ambiguous names":         func(r *Registry) { r.Companies["other"] = Company{Name: "SONY", Aliases: []string{}} },
		"alias against canonical": func(r *Registry) { r.Companies["sony"] = Company{Name: "Sony", Aliases: []string{"SEGA"}} },
		"ambiguous aliases": func(r *Registry) {
			r.Companies["sony"] = Company{Name: "Sony", Aliases: []string{"Example"}}
			r.Companies["sega"] = Company{Name: "Sega", Aliases: []string{"EXAMPLE"}}
		},
		"redundant alias": func(r *Registry) { r.Companies["sony"] = Company{Name: "Sony", Aliases: []string{"sony"}} },
		"blank alias":     func(r *Registry) { r.Companies["sony"] = Company{Name: "Sony", Aliases: []string{""}} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r, _ := Parse(good)
			mutate(r)
			b, _ := json.Marshal(r)
			if _, err := Parse(b); err == nil {
				t.Fatal("bad company data accepted")
			}
		})
	}
	for name, b := range map[string][]byte{
		"duplicate company": bytes.Replace(good, []byte(`"sony": {`), []byte(`"sony": {}, "sony": {`), 1),
		"unknown field":     bytes.Replace(good, []byte(`"aliases": []`), []byte(`"aliases": [], "parentCompany": "other"`), 1),
		"wrong-case field":  bytes.Replace(good, []byte(`"aliases": []`), []byte(`"Aliases": []`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(b, good) {
				t.Fatal("fixture mutation did not apply")
			}
			if _, err := Parse(b); err == nil {
				t.Fatal("invalid company JSON accepted")
			}
		})
	}
	r.Companies["sony"] = Company{Name: "Sony", Aliases: []string{"Test Sony Alias"}}
	s := r.Systems["psx"]
	s.ManufacturerIDs = []string{"sony", "sega"}
	r.Systems["psx"] = s
	if err := r.Validate(); err != nil {
		t.Fatal("multiple explicit manufacturers or unique alias rejected", err)
	}
	s.ManufacturerIDs = []string{}
	r.Systems["psx"] = s
	if err := r.Validate(); err != nil {
		t.Fatal("unknown manufacturer should use empty references", err)
	}
}

func TestCompanyIdentityCompatibility(t *testing.T) {
	old, _ := Parse(fixture(t))
	next, _ := Parse(fixture(t))
	delete(next.Companies, "atari")
	if err := next.Compatible(old); err == nil {
		t.Fatal("removed an established unreferenced company")
	}
	next, _ = Parse(fixture(t))
	next.Companies["sony"] = Company{Name: "Sony display label", Aliases: []string{"Example alias"}}
	if err := next.Compatible(old); err != nil {
		t.Fatal("display/alias edit rejected", err)
	}
	s := next.Systems["psx"]
	s.ManufacturerIDs = []string{"sega"}
	next.Systems["psx"] = s
	if err := next.Compatible(old); err == nil {
		t.Fatal("silently changed grouping relationship")
	}
	s = old.Systems["psx"]
	s.ManufacturerIDs = []string{"sony", "sega"}
	old.Systems["psx"] = s
	s.ManufacturerIDs = []string{"sega", "sony"}
	next.Systems["psx"] = s
	if err := next.Compatible(old); err != nil {
		t.Fatal("reference ordering changed identity", err)
	}
}
