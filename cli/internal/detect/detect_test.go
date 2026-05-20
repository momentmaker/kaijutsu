package detect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestActiveAt(t *testing.T) {
	home := t.TempDir()
	mkdirs(t, home, ".claude", ".gemini/antigravity-cli")

	got := ActiveAt(home)
	want := []string{"claude", "antigravity"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ActiveAt = %v; want %v", got, want)
	}
}

func TestActiveAtEmpty(t *testing.T) {
	home := t.TempDir()
	if got := ActiveAt(home); len(got) != 0 {
		t.Errorf("expected no agents, got %v", got)
	}
}

func mkdirs(t *testing.T, root string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
}
