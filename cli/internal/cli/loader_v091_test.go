package cli

import "testing"

// TestYamlUnmarshal_VersionField verifies the tiny-yaml-peek the
// v0.9.1 resolver uses to extract `version:` from skill.yaml at a
// candidate ref. Tested independently of the network because
// fetchSkillVersionAtRef's network surface is the fetch.Fetcher,
// covered by fetch package tests.
func TestYamlUnmarshal_VersionField(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
		ok   bool
	}{
		{
			name: "happy path — version present",
			body: "name: doc-review\nversion: 0.1.0\nlicense: MIT\n",
			want: "0.1.0",
			ok:   true,
		},
		{
			name: "version field absent",
			body: "name: doc-review\nlicense: MIT\n",
			want: "",
			ok:   true, // unmarshal succeeds; caller checks empty
		},
		{
			name: "malformed YAML",
			body: "name: doc-review\n  version: bad indent\n",
			want: "",
			ok:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sk struct {
				Version string `yaml:"version"`
			}
			err := yamlUnmarshal([]byte(tc.body), &sk)
			if tc.ok && err != nil {
				t.Errorf("expected ok unmarshal; got err: %v", err)
			}
			if !tc.ok && err == nil {
				t.Errorf("expected error; got version=%q", sk.Version)
			}
			if tc.ok && sk.Version != tc.want {
				t.Errorf("version=%q, want %q", sk.Version, tc.want)
			}
		})
	}
}
