package nativeui

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"copiggy/resources"
)

// Translation resources and user-content fixtures may contain any language.
// Product source strings must remain English so missing localization never
// exposes a legacy-language fallback to users.
func TestRuntimeCopyUsesEnglishSourceStrings(t *testing.T) {
	files := token.NewFileSet()
	err := filepath.WalkDir(filepath.Join("..", ".."), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || filepath.Base(path) == "i18n.go" {
			return nil
		}
		source, err := parser.ParseFile(files, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(source, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.ContainsFunc(value, isCJKCopy) {
				t.Errorf("%s: move localized copy to the catalog: %q", files.Position(literal.Pos()), value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func isCJKCopy(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r)
}

func TestStaticTranslationCallsHaveCatalogEntries(t *testing.T) {
	files := token.NewFileSet()
	var paths []string
	err := filepath.WalkDir(filepath.Join("..", ".."), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		source, err := parser.ParseFile(files, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(source, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			argument := -1
			switch function := call.Fun.(type) {
			case *ast.SelectorExpr:
				switch function.Sel.Name {
				case "tr", "reportNativeStatus", "ContentError":
					argument = 0
				case "reportNativeError", "Report":
					argument = 2
				}
			case *ast.Ident:
				if function.Name == "clipboardContentError" {
					argument = 0
				}
			}
			if argument < 0 || argument >= len(call.Args) {
				return true
			}
			literal, ok := call.Args[argument].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			key, _ := strconv.Unquote(literal.Value)
			if _, exists := translations[key]; key != "" && !exists {
				t.Errorf("%s: missing translation for %q", files.Position(literal.Pos()), key)
			}
			return true
		})
	}
	for key, row := range translations {
		if strings.ContainsFunc(key, isCJKCopy) || key != row[0] {
			t.Errorf("catalog key must be its English source copy: %q (English %q)", key, row[0])
		}
	}
}

func TestSearchKeywordsCoverSupportedLanguages(t *testing.T) {
	var terms map[string]map[string]string
	if err := json.Unmarshal(resources.SearchTerms, &terms); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"text", "json", "link", "image"} {
		for _, language := range supportedLanguages {
			if strings.TrimSpace(terms[kind][language.Code]) == "" {
				t.Errorf("missing %s search keywords for %s", language.Code, kind)
			}
		}
	}
}
