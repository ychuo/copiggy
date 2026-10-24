package clipmodel

import "testing"

func TestFilePathRecognitionAcceptsLiteralQuotedLocalPaths(t *testing.T) {
	for _, tc := range []struct {
		raw, path, name string
		home            bool
	}{
		{`"/Users/alex/My File.txt"`, "/Users/alex/My File.txt", "My File.txt", false},
		{`'/Users/alex/My File.txt'`, "/Users/alex/My File.txt", "My File.txt", false},
		{`"~/Documents/My File.txt"`, "~/Documents/My File.txt", "My File.txt", true},
		{`'~/Documents/My File.txt'`, "~/Documents/My File.txt", "My File.txt", true},
		{"  \"/tmp/trailing space \" \n", "/tmp/trailing space ", "trailing space ", false},
		{`"/tmp/O'Brien.txt"`, "/tmp/O'Brien.txt", "O'Brien.txt", false},
		{`'/tmp/"draft".txt'`, `/tmp/"draft".txt`, `"draft".txt`, false},
		{`'/tmp/My\ File.txt'`, `/tmp/My\ File.txt`, `My\ File.txt`, false},
		{`"/tmp/file$(touch demo).txt"`, "/tmp/file$(touch demo).txt", "file$(touch demo).txt", false},
		{`"/tmp/100%20literal.txt"`, "/tmp/100%20literal.txt", "100%20literal.txt", false},
		{`"/tmp/./report.txt"`, "/tmp/report.txt", "report.txt", false},
		{"FILE:///Users/alex/My%20File.txt", "/Users/alex/My File.txt", "My File.txt", false},
		{"file://LOCALHOST/Users/alex/My%20File.txt", "/Users/alex/My File.txt", "My File.txt", false},
		{`'FiLe://LocalHost/tmp/a%23b%20'`, "/tmp/a#b ", "a#b ", false},
		{`"file:///tmp/quote%22inside.txt"`, `/tmp/quote"inside.txt`, `quote"inside.txt`, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			analysis := Analyze(tc.raw)
			file := analysis.FilePath
			if analysis.Type != TypeFilePath || file == nil || file.Path != tc.path || file.Name != tc.name || file.HomeRelative != tc.home {
				t.Fatalf("path = %+v, want %q / %q / home %v", file, tc.path, tc.name, tc.home)
			}
			store := NewStore(1)
			item, ok := store.CaptureProtected(tc.raw, "fixture.editor", []string{"fixture.editor"})
			if !ok || item.Type != TypeFilePath || item.Text != tc.raw || !item.Allows("fixture.editor") || item.Allows("other.app") {
				t.Fatalf("recognition changed original text or scope: %+v", item)
			}
		})
	}
}

func TestFilePathRecognitionRejectsMalformedAndNonlocalWrappers(t *testing.T) {
	for _, raw := range []string{
		`"`, `'`, `""`, `''`, `"/tmp/a`, `'/tmp/a`, `"/tmp/a'`, `'/tmp/a"`,
		`"/tmp/a" "/tmp/b"`, `'/tmp/a' '/tmp/b'`, `"/tmp/a" trailing`, `""/tmp/a""`, `"'/tmp/a'"`,
		`" /tmp/a"`, `' ~/a'`, `"relative/a.txt"`, `'../a'`, `"$HOME/a"`, `"~other/a"`, `"C:\Users\alex\a.txt"`,
		`"//remote/a"`, `"/tmp/../private"`, `'~/../private'`, `"/tmp/.."`, `"/"`, `'~/'`,
		`"file://remote.example/a"`, `'FILE://user@localhost/a'`, `"file://localhost:80/a"`,
		`"file:relative"`, `"file:~/a"`, `"file:///tmp/%2e%2e/private"`, `"file:///tmp/a?download=1"`,
		`"file:///tmp/a?"`, `"file:///tmp/a#fragment"`, `"file:///tmp/a#"`, `"file:///tmp/a%00b"`,
		`"file:///tmp/a%09b"`, `"file:///tmp/a%0Ab"`, `"file:///tmp/a%1Bb"`,
		"\"/tmp/a\tb\"", "'/tmp/a\nb'", "\"/tmp/a\x00b\"", "\"/tmp/a\x1bb\"", "\"/tmp/a\x7fb\"",
	} {
		t.Run(raw, func(t *testing.T) {
			if analysis := Analyze(raw); analysis.Type != TypeText || analysis.FilePath != nil {
				t.Fatalf("malformed or nonlocal quoted path accepted: %q => %+v", raw, analysis)
			}
		})
	}
}
