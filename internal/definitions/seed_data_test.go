package definitions

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strconv"
	"testing"
)

// Frozen from ROMD a601c4e7da7d9e699a18237417f6d749bc0290c3:
// PlatformSeeder.cs, PlatformIds.cs, SeedData/regions.json and languages.json.
// This independent legacy layout guards the initial migration's full contents,
// not just a few familiar systems or row counts.
func TestExistingROMDSeedParity(t *testing.T) {
	var baseline struct {
		Systems []struct {
			ShortName, Name string
			Manufacturer    *string
			IgdbID          int
			Aliases         []string
		}
		Regions   []Region
		Languages []Language
	}
	raw, err := os.ReadFile("testdata/romd-seed-baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &baseline); err != nil {
		t.Fatal(err)
	}
	r, err := LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Systems) != len(baseline.Systems) || len(r.Regions) != len(baseline.Regions) || len(r.Languages) != len(baseline.Languages) {
		t.Fatal("seed coverage changed")
	}
	for _, old := range baseline.Systems {
		s, ok := r.Systems[old.ShortName]
		if !ok || s.Name != old.Name || !slices.Equal(s.Aliases, old.Aliases) || len(s.ProviderMappings) != 1 || s.ProviderMappings["igdb"] != strconv.Itoa(old.IgdbID) {
			t.Fatalf("system seed drift: %s", old.ShortName)
		}
		if old.Manufacturer == nil {
			if len(s.ManufacturerIDs) != 0 {
				t.Fatal("invented manufacturer")
			}
		} else if len(s.ManufacturerIDs) != 1 || r.Companies[s.ManufacturerIDs[0]].Name != *old.Manufacturer {
			t.Fatalf("manufacturer drift: %s", old.ShortName)
		}
	}
	for _, old := range baseline.Regions {
		found := false
		for _, current := range r.Regions {
			if reflect.DeepEqual(current, old) {
				found = true
			}
		}
		if !found {
			t.Fatalf("region seed drift: %s", old.Name)
		}
	}
	for _, old := range baseline.Languages {
		if !reflect.DeepEqual(r.Languages[old.Code], old) {
			t.Fatalf("language seed drift: %s", old.Code)
		}
	}
}

func TestLegacySnapshotUpgradeAndNoDowngrade(t *testing.T) {
	old, err := Load("testdata/schema-1.json")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Parse(raw)
	if err != nil || !reflect.DeepEqual(restored, old) {
		t.Fatal("legacy roundtrip", err)
	}
	next, err := LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Compatible(old); err != nil {
		t.Fatal("cannot upgrade published snapshot", err)
	}
	if err := old.Compatible(next); err == nil {
		t.Fatal("allowed schema downgrade")
	}
	for _, category := range []string{"regions", "languages"} {
		candidate, _ := Parse(fixture(t))
		if category == "regions" {
			delete(candidate.Regions, "world")
		} else {
			delete(candidate.Languages, "en")
		}
		if err := candidate.Compatible(next); err == nil {
			t.Fatal("removed stable taxonomy identity", category)
		}
	}
}

func TestSeedValidationRejectsAmbiguity(t *testing.T) {
	cases := map[string]func(*Registry){
		"system alias":        func(r *Registry) { s := r.Systems["psx"]; s.Aliases = append(s.Aliases, "SNES"); r.Systems["psx"] = s },
		"system ID alias":     func(r *Registry) { s := r.Systems["psx"]; s.Aliases = append(s.Aliases, "GC"); r.Systems["psx"] = s },
		"duplicate own alias": func(r *Registry) { s := r.Systems["psx"]; s.Aliases = append(s.Aliases, "psx"); r.Systems["psx"] = s },
		"null system aliases": func(r *Registry) { s := r.Systems["psx"]; s.Aliases = nil; r.Systems["psx"] = s },
		"duplicate mapping":   func(r *Registry) { r.Systems["psx"].ProviderMappings["igdb"] = "18" },
		"invalid mapping key": func(r *Registry) { r.Systems["psx"].ProviderMappings["../igdb"] = "7" },
		"blank mapping":       func(r *Registry) { r.Systems["psx"].ProviderMappings["igdb"] = " " },
		"region alias": func(r *Registry) {
			v := r.Regions["world"]
			v.Aliases = append(v.Aliases, "USA")
			r.Regions["world"] = v
		},
		"region ID":         func(r *Registry) { r.Regions["../bad"] = r.Regions["world"] },
		"region sort order": func(r *Registry) { v := r.Regions["world"]; v.SortOrder = -1; r.Regions["world"] = v },
		"empty regions":     func(r *Registry) { r.Regions = map[string]Region{} },
		"language code":     func(r *Registry) { v := r.Languages["en"]; v.Code = "ja"; r.Languages["en"] = v },
		"language alias": func(r *Registry) {
			v := r.Languages["en"]
			v.Aliases = append(v.Aliases, "Japanese")
			r.Languages["en"] = v
		},
		"null language aliases": func(r *Registry) { v := r.Languages["en"]; v.Aliases = nil; r.Languages["en"] = v },
		"null languages":        func(r *Registry) { r.Languages = nil },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := Parse(fixture(t))
			if err != nil {
				t.Fatal(err)
			}
			change(r)
			if err := r.Validate(); err == nil {
				t.Fatal("invalid seed accepted")
			}
			b, _ := json.Marshal(r)
			if _, err := Parse(b); err == nil {
				t.Fatal("invalid seed JSON accepted")
			}
		})
	}
}

func TestExistingProviderMappingCannotBeReassigned(t *testing.T) {
	old, err := LoadSource("../../definitions")
	if err != nil {
		t.Fatal(err)
	}
	next, err := Parse(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	next.Systems["psx"].ProviderMappings["igdb"] = "999999"
	if err := next.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := next.Compatible(old); err == nil {
		t.Fatal("silently reassigned existing provider identity")
	}
	delete(next.Systems["psx"].ProviderMappings, "igdb")
	if err := next.Compatible(old); err == nil {
		t.Fatal("silently removed existing provider identity")
	}
}
