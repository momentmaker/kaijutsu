package source

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in, owner, repo string
		err             bool
	}{
		{"momentmaker/kaijutsu", "momentmaker", "kaijutsu", false},
		{"github.com/momentmaker/kaijutsu", "momentmaker", "kaijutsu", false},
		{"https://github.com/momentmaker/kaijutsu", "momentmaker", "kaijutsu", false},
		{"https://github.com/momentmaker/kaijutsu.git", "momentmaker", "kaijutsu", false},
		{"http://github.com/owner/repo", "owner", "repo", false},
		{"", "", "", true},
		{"justonepart", "", "", true},
		{"too/many/parts", "", "", true},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if c.err {
			if err == nil {
				t.Errorf("Parse(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got.Owner != c.owner || got.Repo != c.repo {
			t.Errorf("Parse(%q) = %v, want %s/%s", c.in, got, c.owner, c.repo)
		}
	}
}

func TestURLs(t *testing.T) {
	s := &Source{Host: "github.com", Owner: "momentmaker", Repo: "kaijutsu"}
	if got, want := s.String(), "momentmaker/kaijutsu"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got, want := s.APIBase(), "https://api.github.com/repos/momentmaker/kaijutsu"; got != want {
		t.Errorf("APIBase() = %q", got)
	}
	if got, want := s.TarballURL("abc"), "https://codeload.github.com/momentmaker/kaijutsu/tar.gz/abc"; got != want {
		t.Errorf("TarballURL() = %q", got)
	}
}
