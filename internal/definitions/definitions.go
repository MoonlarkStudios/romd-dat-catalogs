// Package definitions owns the shared system identity and catalog registry.
// Registry data selects known adapters; it cannot supply URLs or executable code.
package definitions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxBytes = 1 << 20

type System struct {
	Name            string   `json:"name"`
	ManufacturerIDs []string `json:"manufacturerIds"`
}
type Company struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}
type Validation struct {
	MinimumGames int `json:"minimumGames"`
	MinimumROMs  int `json:"minimumRoms"`
}
type Catalog struct {
	SystemID         string     `json:"systemId"`
	Provider         string     `json:"provider"`
	ProviderSystemID string     `json:"providerSystemId"`
	Representation   string     `json:"representation"`
	ExpectedName     string     `json:"expectedName"`
	Validation       Validation `json:"validation"`
}
type Registry struct {
	Companies     map[string]Company `json:"companies"`
	SchemaVersion int                `json:"schemaVersion"`
	Systems       map[string]System  `json:"systems"`
	Catalogs      map[string]Catalog `json:"catalogs"`
}

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func label(s string) bool {
	return len(s) > 0 && len(s) <= 200 && strings.TrimSpace(s) == s && strings.IndexFunc(s, unicode.IsControl) < 0
}

func Load(path string) (*Registry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	return Parse(b)
}
func Parse(b []byte) (*Registry, error) {
	if len(b) > MaxBytes || !utf8.Valid(b) {
		return nil, errors.New("definitions must be bounded UTF-8 JSON")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if err := uniqueValue(d, 0); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errors.New("trailing definitions data")
	}
	var raw map[string]json.RawMessage
	if err := object(b, &raw, "schemaVersion", "systems", "catalogs", "companies"); err != nil {
		return nil, err
	}
	for _, section := range []string{"systems", "catalogs", "companies"} {
		var entries map[string]json.RawMessage
		if err := json.Unmarshal(raw[section], &entries); err != nil {
			return nil, err
		}
		for _, entry := range entries {
			var fields map[string]json.RawMessage
			if section == "systems" {
				if err := object(entry, &fields, "name", "manufacturerIds"); err != nil {
					return nil, err
				}
			} else if section == "companies" {
				if err := object(entry, &fields, "name", "aliases"); err != nil {
					return nil, err
				}
			} else {
				if err := object(entry, &fields, "systemId", "provider", "providerSystemId", "representation", "expectedName", "validation"); err != nil {
					return nil, err
				}
				var limits map[string]json.RawMessage
				if err := object(fields["validation"], &limits, "minimumGames", "minimumRoms"); err != nil {
					return nil, err
				}
			}
		}
	}
	var r Registry
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return &r, nil
}

// Exact names and required fields avoid encoding/json's case-insensitive aliases.
func object(b []byte, out *map[string]json.RawMessage, keys ...string) error {
	if err := json.Unmarshal(b, out); err != nil {
		return err
	}
	if len(*out) != len(keys) {
		return errors.New("missing or unknown definition field")
	}
	for _, key := range keys {
		v, ok := (*out)[key]
		if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return fmt.Errorf("required definition field %s", key)
		}
	}
	return nil
}

// Check every object before unmarshalling: JSON maps otherwise silently replace
// duplicate keys, including escaped spellings of an existing key.
func uniqueValue(d *json.Decoder, depth int) error {
	if depth > 32 {
		return errors.New("definitions nesting limit")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate definition key")
			}
			seen[name] = true
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	case json.Delim('['):
		for d.More() {
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	return nil
}
func (r *Registry) Validate() error {
	if r == nil || r.SchemaVersion != 1 || len(r.Companies) == 0 || len(r.Companies) > 10000 || len(r.Systems) == 0 || len(r.Systems) > 1000 || len(r.Catalogs) == 0 || len(r.Catalogs) > 1000 {
		return errors.New("unsupported or empty definitions")
	}
	companyNames := map[string]string{}
	for id, c := range r.Companies {
		if !idPattern.MatchString(id) || len(id) > 64 || !label(c.Name) || c.Aliases == nil || len(c.Aliases) > 100 {
			return fmt.Errorf("invalid company %q", id)
		}
		names := append([]string{c.Name}, c.Aliases...)
		for _, name := range names {
			if !label(name) {
				return fmt.Errorf("invalid company name/alias for %q", id)
			}
			normalized := strings.ToLower(name)
			if prior, exists := companyNames[normalized]; exists {
				return fmt.Errorf("duplicate or ambiguous company name/alias for %q and %q", prior, id)
			}
			companyNames[normalized] = id
		}
	}
	for id, s := range r.Systems {
		if !idPattern.MatchString(id) || len(id) > 64 || !label(s.Name) || s.ManufacturerIDs == nil {
			return fmt.Errorf("invalid system %q", id)
		}
		seen := map[string]bool{}
		for _, companyID := range s.ManufacturerIDs {
			if _, exists := r.Companies[companyID]; !exists {
				return fmt.Errorf("unknown manufacturer %q for system %q", companyID, id)
			}
			if seen[companyID] {
				return fmt.Errorf("duplicate manufacturer %q for system %q", companyID, id)
			}
			seen[companyID] = true
		}
	}
	mappings := map[string]bool{}
	for id, c := range r.Catalogs {
		if _, ok := r.Systems[c.SystemID]; !ok {
			return fmt.Errorf("unknown system for catalog %q", id)
		}
		if c.Provider != "redump" || c.Representation != "discs" || !idPattern.MatchString(c.ProviderSystemID) || len(c.ProviderSystemID) > 64 || !label(c.ExpectedName) || c.Validation.MinimumGames < 1 || c.Validation.MinimumROMs < 1 {
			return fmt.Errorf("invalid catalog %q", id)
		}
		if id != c.Provider+"/"+c.SystemID+"/"+c.Representation {
			return fmt.Errorf("catalog ID must match provider/system/representation: %q", id)
		}
		key := c.Provider + "/" + c.ProviderSystemID + "/" + c.Representation
		if mappings[key] {
			return fmt.Errorf("conflicting provider mapping %q", key)
		}
		mappings[key] = true
	}
	return nil
}

// Compatible permits display-label and validation-policy edits, but never silent
// removal or reassignment of established IDs. No automatic migration is provided.
func (r *Registry) Compatible(previous *Registry) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if previous == nil {
		return nil
	}
	for id := range previous.Companies {
		if _, ok := r.Companies[id]; !ok {
			return fmt.Errorf("company %q removed; explicit migration required", id)
		}
	}
	for id, old := range previous.Systems {
		if _, ok := r.Systems[id]; !ok {
			return fmt.Errorf("system %q removed; explicit migration required", id)
		}
		before := slices.Clone(old.ManufacturerIDs)
		after := slices.Clone(r.Systems[id].ManufacturerIDs)
		slices.Sort(before)
		slices.Sort(after)
		if !slices.Equal(before, after) {
			return fmt.Errorf("system %q manufacturer relationship changed; explicit migration required", id)
		}
	}
	for id, old := range previous.Catalogs {
		c, ok := r.Catalogs[id]
		if !ok || c.SystemID != old.SystemID || c.Provider != old.Provider || c.ProviderSystemID != old.ProviderSystemID || c.Representation != old.Representation || c.ExpectedName != old.ExpectedName {
			return fmt.Errorf("catalog %q identity changed; explicit migration required", id)
		}
	}
	return nil
}
