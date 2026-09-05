package publisher

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path"
	"strings"
)

const MaxInput = 16 << 20
const MaxDocument = 32 << 20
const MaxMetadata = 16 << 20

type CandidateError string

func (e CandidateError) Error() string { return string(e) }

type Counts struct {
	Games int `json:"games"`
	ROMs  int `json:"roms"`
}

func readBounded(r io.Reader, limit int64) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, CandidateError("size_limit")
	}
	return b, nil
}
func readFile(name string, limit int64) ([]byte, error) {
	f, e := os.Open(name)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return readBounded(f, limit)
}

// Document validates tokens without constructing a DOM. Original bytes are preserved.
// encoding/xml does not retrieve external DTDs; custom entities are never enabled.
func Document(raw []byte, expected string) ([]byte, Counts, error) {
	c := Counts{}
	bad := func(code string) ([]byte, Counts, error) { return nil, Counts{}, CandidateError(code) }
	if len(raw) > MaxInput {
		return bad("size_limit")
	}
	z, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if e == nil {
		if len(z.File) != 1 {
			return bad("ambiguous_archive")
		}
		f := z.File[0]
		if !f.Mode().IsRegular() || strings.HasPrefix(f.Name, "/") || strings.Contains(f.Name, "\\") || strings.ToLower(path.Ext(f.Name)) != ".dat" || f.Flags&1 != 0 {
			return bad("unsafe_archive")
		}
		for _, part := range strings.Split(f.Name, "/") {
			if part == ".." {
				return bad("unsafe_archive")
			}
		}
		if f.UncompressedSize64 > MaxDocument {
			return bad("size_limit")
		}
		stream, e := f.Open()
		if e != nil {
			return bad("invalid_dat")
		}
		raw, e = readBounded(stream, MaxDocument)
		stream.Close()
		if e != nil {
			var ce CandidateError
			if errors.As(e, &ce) {
				return bad(string(ce))
			}
			return bad("invalid_dat")
		}
	}
	if len(raw) > MaxDocument {
		return bad("size_limit")
	}
	// A UTF-8 BOM is valid XML; strip it only from the parser's view, never the artifact.
	d := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})))
	stack := []string{}
	headers, roots := 0, 0
	name := ""
	names := map[string]bool{}
	for {
		token, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return bad("invalid_dat")
		}
		switch t := token.(type) {
		case xml.Directive:
			if bytes.Contains(t, []byte("ENTITY")) {
				return bad("invalid_dat")
			}
		case xml.StartElement:
			if len(stack) >= 128 || t.Name.Space != "" {
				return bad("invalid_dat")
			}
			attributes := make(map[xml.Name]bool, len(t.Attr))
			for _, attribute := range t.Attr {
				if attributes[attribute.Name] {
					return bad("invalid_dat")
				}
				attributes[attribute.Name] = true
			}
			stack = append(stack, t.Name.Local)
			if len(stack) == 1 {
				roots++
				if roots != 1 || t.Name.Local != "datafile" {
					return bad("invalid_dat")
				}
			}
			switch {
			case len(stack) == 2 && stack[1] == "header":
				headers++
			case len(stack) == 2 && stack[1] == "game":
				game := ""
				for _, a := range t.Attr {
					if a.Name.Local == "name" && a.Name.Space == "" {
						game = a.Value
					}
				}
				if game == "" || names[game] {
					return bad("invalid_game_identity")
				}
				names[game] = true
				c.Games++
			case len(stack) == 3 && stack[1] == "game" && stack[2] == "rom":
				c.ROMs++
			}
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 && strings.TrimSpace(string(t)) != "" {
				return bad("invalid_dat")
			}
			if len(stack) == 3 && stack[1] == "header" && stack[2] == "name" {
				name += string(t)
			}
		}
	}
	if roots != 1 || headers != 1 || len(stack) != 0 {
		return bad("invalid_dat")
	}
	if name != expected {
		return bad("identity_mismatch")
	}
	if c.Games == 0 {
		return bad("empty_dat")
	}
	return raw, c, nil
}
