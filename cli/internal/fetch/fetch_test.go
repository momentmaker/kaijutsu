package fetch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractStripsPathTraversal(t *testing.T) {
	tgz := makeTarball(t, []tarEntry{
		{name: "../evil.txt", body: "x"},
	})
	tmp := t.TempDir()
	if _, err := Extract(tgz, tmp); err == nil {
		t.Error("expected error rejecting traversal entry")
	}
}

func TestExtractRoundTrip(t *testing.T) {
	tgz := makeTarball(t, []tarEntry{
		// GitHub tarballs lead with a PAX global header that must be
		// skipped, otherwise it gets treated as the top-level dir.
		{name: "pax_global_header", paxGlobal: true},
		{name: "owner-repo-abc/", isDir: true},
		{name: "owner-repo-abc/skills/", isDir: true},
		{name: "owner-repo-abc/skills/decide/", isDir: true},
		{name: "owner-repo-abc/skills/decide/SKILL.md", body: "# decide"},
	})
	tmp := t.TempDir()
	root, err := Extract(tgz, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(root, "owner-repo-abc") {
		t.Errorf("unexpected root %q", root)
	}
	body, err := os.ReadFile(filepath.Join(root, "skills", "decide", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "# decide" {
		t.Errorf("body = %q", string(body))
	}
}

type tarEntry struct {
	name      string
	body      string
	isDir     bool
	paxGlobal bool
}

func makeTarball(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		var h *tar.Header
		switch {
		case e.paxGlobal:
			h = &tar.Header{Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "global"}}
		default:
			h = &tar.Header{Name: e.name, Mode: 0644, Size: int64(len(e.body))}
		}
		switch {
		case e.paxGlobal:
			// already configured above
		case e.isDir:
			h.Typeflag = tar.TypeDir
			h.Size = 0
			h.Mode = 0755
		default:
			h.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if !e.isDir && !e.paxGlobal {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
