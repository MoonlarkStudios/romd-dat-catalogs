package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	var out, errOut bytes.Buffer
	root := filepath.Join(t.TempDir(), "output")
	if e := run([]string{"--output", root, "../../fixtures/manifest.json"}, &out, &errOut); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), `"failed":0`) || !strings.Contains(out.String(), `"trust":"unsigned-development"`) {
		t.Fatal(out.String())
	}
}
func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{}, {"--output", "unused"}, {"--unknown"}} {
		var out bytes.Buffer
		if e := run(args, &out, &out); e == nil {
			t.Fatal("accepted invalid args")
		}
	}
}
