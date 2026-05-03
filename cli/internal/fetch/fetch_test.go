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
	name  string
	body  string
	isDir bool
}

func makeTarball(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: 0644, Size: int64(len(e.body))}
		if e.isDir {
			h.Typeflag = tar.TypeDir
			h.Size = 0
			h.Mode = 0755
		} else {
			h.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if !e.isDir {
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
