package definitions

import (
	"errors"
	"fmt"
	"strings"
)

func validID(id string) bool { return len(id) <= 64 && idPattern.MatchString(id) }

// Names and aliases resolve case-insensitively within each category. An alias
// may match its own stable ID or language code (as ROMD's existing seeds do),
// but may never make two different entries resolve to the same token.
func addNames(seen map[string]string, id, name string, aliases []string, tokens ...string) error {
	if !validID(id) || !label(name) || aliases == nil || len(aliases) > 100 {
		return fmt.Errorf("invalid names for %q", id)
	}
	own := map[string]bool{}
	for _, alias := range aliases {
		key := strings.ToLower(alias)
		if !label(alias) || own[key] {
			return fmt.Errorf("invalid or duplicate alias for %q", id)
		}
		own[key] = true
	}
	names := append([]string{name}, aliases...)
	names = append(names, tokens...)
	for _, token := range names {
		key := strings.ToLower(token)
		if prior, ok := seen[key]; ok && prior != id {
			return fmt.Errorf("ambiguous name/alias %q for %q and %q", token, prior, id)
		}
		seen[key] = id
	}
	return nil
}

func (r *Registry) validateSeedData() error {
	if r.SchemaVersion == 1 {
		if r.Regions != nil || r.Languages != nil {
			return errors.New("schema 1 cannot contain taxonomy data")
		}
		for _, s := range r.Systems {
			if s.Aliases != nil || s.ProviderMappings != nil {
				return errors.New("schema 1 cannot contain system seed extensions")
			}
		}
		return nil
	}
	if len(r.Regions) == 0 || len(r.Regions) > 1000 || len(r.Languages) == 0 || len(r.Languages) > 1000 {
		return errors.New("missing or oversized taxonomy definitions")
	}
	systems, regions, languages, mappings := map[string]string{}, map[string]string{}, map[string]string{}, map[string]string{}
	for id, s := range r.Systems {
		if err := addNames(systems, id, s.Name, s.Aliases, id); err != nil {
			return err
		}
		if s.ProviderMappings == nil || len(s.ProviderMappings) > 100 {
			return fmt.Errorf("invalid provider mappings for %q", id)
		}
		for provider, value := range s.ProviderMappings {
			if !validID(provider) || !label(value) {
				return fmt.Errorf("invalid provider mapping for %q", id)
			}
			key := provider + "/" + strings.ToLower(value)
			if prior, ok := mappings[key]; ok {
				return fmt.Errorf("ambiguous provider mapping for %q and %q", prior, id)
			}
			mappings[key] = id
		}
	}
	for id, region := range r.Regions {
		if region.SortOrder < 0 || region.SortOrder > 100000 {
			return fmt.Errorf("invalid region sort order for %q", id)
		}
		if err := addNames(regions, id, region.Name, region.Aliases); err != nil {
			return err
		}
	}
	for id, language := range r.Languages {
		if language.Code != id || language.SortOrder < 0 || language.SortOrder > 100000 {
			return fmt.Errorf("invalid language code/sort order for %q", id)
		}
		if err := addNames(languages, id, language.Name, language.Aliases, language.Code); err != nil {
			return err
		}
	}
	return nil
}
