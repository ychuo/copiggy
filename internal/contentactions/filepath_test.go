package contentactions

import (
	"errors"
	"testing"
)

func TestQuotedFilePathTargetsPreserveLiteralCharacters(t *testing.T) {
	for _, tc := range []struct{ raw, path, home, resolved string }{
		{`"/tmp/a b.txt"`, "/tmp/a b.txt", "", "/tmp/a b.txt"},
		{`'~/a b.txt'`, "~/a b.txt", "/Users/alex", "/Users/alex/a b.txt"},
		{`'FiLe://LocalHost/tmp/a%23b%20'`, "/tmp/a#b ", "", "/tmp/a#b "},
		{`"/tmp/$(touch demo).txt"`, "/tmp/$(touch demo).txt", "", "/tmp/$(touch demo).txt"},
		{`'/tmp/My\ File.txt'`, `/tmp/My\ File.txt`, "", `/tmp/My\ File.txt`},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			target, err := OpenTarget(tc.raw)
			if err != nil || target.Path != tc.path || target.URL != "" || target.Calendar != nil {
				t.Fatalf("quoted path target = %+v, %v", target, err)
			}
			resolved, err := ResolvePath(tc.raw, tc.home)
			if err != nil || resolved != tc.resolved {
				t.Fatalf("quoted path resolved to %q, %v; want %q", resolved, err, tc.resolved)
			}
		})
	}
	for _, raw := range []string{`"relative.txt"`, `"/tmp/a" trailing`, `"FILE://remote.example/a"`, `"/tmp/../private"`, `"file:///tmp/a%0Ab"`} {
		if _, err := ResolvePath(raw, "/Users/alex"); !errors.Is(err, ErrInvalidPath) {
			t.Errorf("invalid wrapped path produced an external target: %q", raw)
		}
	}
}
