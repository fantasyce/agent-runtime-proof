package expectation

import "testing"

func TestIncludesUsesRecursiveGlobAndExcludePrecedence(t *testing.T) {
	resolved := Resolved{}
	resolved.Value.Artifact.Include = []string{"bin/**", "manifest.json"}
	resolved.Value.Artifact.Exclude = []string{"bin/cache/**"}

	cases := map[string]bool{
		"bin/runtime":          true,
		"bin/lib/module.js":    true,
		"manifest.json":        true,
		"bin/cache/secret.bin": false,
		"other.txt":            false,
		"../escape":            false,
		"/absolute":            false,
	}
	for name, want := range cases {
		if got := resolved.Includes(name); got != want {
			t.Errorf("Includes(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestMatchPatternSupportsQuestionAndCharacterClass(t *testing.T) {
	for _, test := range []struct {
		pattern string
		path    string
		want    bool
	}{
		{"lib/file?.[jt]s", "lib/file1.js", true},
		{"lib/file?.[jt]s", "lib/file2.ts", true},
		{"lib/file?.[jt]s", "lib/file10.js", false},
		{"**/manifest.json", "nested/deep/manifest.json", true},
		{"assets/**/icon.png", "assets/icon.png", true},
	} {
		if got := matchPattern(test.pattern, test.path); got != test.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", test.pattern, test.path, got, test.want)
		}
	}
}
