package nointro

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"strconv"
	"strings"

	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/definitions"
	"github.com/MoonlarkStudios/romd-dat-catalogs/internal/publisher"
)

// Validate preserves the exact extracted bytes. Counts are anomaly floors, not
// authentication or proof of upstream database completeness. Header ID prevents
// a similarly named variant from being routed to this reviewed system.
func Validate(raw []byte, c definitions.Catalog) ([]byte, publisher.Counts, error) {
	doc, counts, err := publisher.Document(raw, c.ExpectedName)
	if err != nil {
		return nil, counts, err
	}
	bad := errors.New("invalid No-Intro catalog identity or ROM structure")
	if counts.Games < c.Validation.MinimumGames || counts.ROMs < c.Validation.MinimumROMs {
		return nil, counts, bad
	}
	var parsed struct {
		Header struct {
			IDs      []string `xml:"id"`
			Versions []string `xml:"version"`
			Homepage string   `xml:"homepage"`
		} `xml:"header"`
		Games []struct {
			ROMs []struct {
				Name   string `xml:"name,attr"`
				Size   string `xml:"size,attr"`
				CRC    string `xml:"crc,attr"`
				MD5    string `xml:"md5,attr"`
				SHA1   string `xml:"sha1,attr"`
				SHA256 string `xml:"sha256,attr"`
				Status string `xml:"status,attr"`
			} `xml:"rom"`
		} `xml:"game"`
	}
	if err = xml.Unmarshal(bytes.TrimPrefix(doc, []byte{0xef, 0xbb, 0xbf}), &parsed); err != nil {
		return nil, counts, err
	}
	if len(parsed.Header.IDs) != 1 || parsed.Header.IDs[0] != c.ProviderSystemID || len(parsed.Header.Versions) != 1 || strings.TrimSpace(parsed.Header.Versions[0]) == "" || parsed.Header.Homepage != "No-Intro" {
		return nil, counts, bad
	}
	for _, game := range parsed.Games {
		if len(game.ROMs) == 0 {
			return nil, counts, bad
		}
		names := map[string]bool{}
		for _, rom := range game.ROMs {
			if rom.Name == "" || names[rom.Name] {
				return nil, counts, bad
			}
			names[rom.Name] = true
			if _, err := strconv.ParseUint(rom.Size, 10, 64); err != nil && !(rom.Size == "" && rom.Status == "nodump") {
				return nil, counts, bad
			}
			if rom.Status != "" && rom.Status != "verified" && rom.Status != "baddump" && rom.Status != "nodump" {
				return nil, counts, bad
			}
			for length, value := range map[int]string{8: rom.CRC, 32: rom.MD5, 40: rom.SHA1, 64: rom.SHA256} {
				// Nodump entries may omit hashes; never synthesize or treat them as verified.
				if value == "" && (rom.Status == "nodump" || length == 64) {
					continue
				}
				if len(value) != length {
					return nil, counts, bad
				}
				if _, err := hex.DecodeString(value); err != nil {
					return nil, counts, bad
				}
			}
		}
	}
	return doc, counts, nil
}
