package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPlatformTokenSource(t *testing.T) {
	cases := map[string]string{
		"server: https://qa.example.com:6443\n      tokenFile: /tokens/qa/token\n      caFile: /tokens/qa/ca.crt": "",
		"tokenFile: /tokens/qa/token":                                               "need server",
		"server: http://qa.example.com":                                             "https://",
		"server: https://qa.example.com":                                            "needs tokenFile",
		"server: https://qa.example.com\n      tokenFile: t\n      inCluster: true": "cannot be combined",
	}
	for env, want := range cases {
		path := filepath.Join(t.TempDir(), "pbom.yaml")
		doc := "apiVersion: pbom.dev/v1alpha1\nkind: Platform\nmetadata:\n  name: p\nspec:\n  environments:\n    - name: qa\n      " + env + "\n"
		if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadPlatform(path)
		switch {
		case want == "" && err != nil:
			t.Errorf("%q: unexpected error %v", env, err)
		case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
			t.Errorf("%q: error %v, want %q", env, err, want)
		}
	}
}
