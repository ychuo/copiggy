package nativeui

import (
	"bytes"
	"context"
	"log"
	"net/url"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/clipboard"
	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func nativePathSourceFixture(t *testing.T, finder bool, paths ...string) (*application, clipmodel.Item) {
	t.Helper()
	s := nativeViewFixture(t)
	s.language = "en"
	s.store = clipboard.NewHistory(nil, 30)
	// Both copying a file and copying its pathname can originate in Finder.
	// Keep provenance equal when comparing their content surfaces, and seed an
	// empty icon cache so no native application lookup is needed by the fixture.
	s.appIcons["com.apple.finder"] = nil
	text := strings.Join(paths, "\n")
	data := clipmodel.TextData(text)
	if finder {
		data = nil
		for _, local := range paths {
			uri := (&url.URL{Scheme: "file", Path: local}).String()
			data = append(data, []clipmodel.Representation{
				{Format: clipmodel.FileListFormat, Bytes: []byte(uri)},
				{Format: clipmodel.NativeFileURLFormat, Bytes: []byte(uri)},
				{Format: clipmodel.TextFormat, Bytes: []byte(path.Base(local))},
			})
		}
	}
	item := nativeCapture(t, s, data, clipmodel.Source{Name: "Finder", BundleID: "com.apple.finder"})
	if item.Type != clipmodel.Analyze(text).Type || item.Text != text {
		t.Fatalf("file source did not become the same full path text: type=%s text=%q", item.Type, item.Text)
	}
	s.filter()
	s.preview = true
	return s, item
}

func TestFinderPathsUseTheSameCardAndPreviewAsCopiedText(t *testing.T) {
	for _, paths := range [][]string{
		{"/Volumes/Offline/My File.pdf"},
		{"/Volumes/Offline/项目说明与界面设计最终稿.pdf"},
		{"/Volumes/Offline/Brief.pdf", "/Volumes/Offline/操作説明.md"},
	} {
		for _, language := range supportedLanguages {
			for _, dark := range []bool{false, true} {
				t.Run(strings.Join(paths, ",")+"/"+language.Code, func(t *testing.T) {
					finder, fileItem := nativePathSourceFixture(t, true, paths...)
					direct, textItem := nativePathSourceFixture(t, false, paths...)
					finder.language, direct.language = language.Code, language.Code
					finder.viewState.Dark, direct.viewState.Dark = dark, dark
					beforeFinder, beforeDirect := finder.store.Records(), direct.store.Records()
					for _, preview := range []bool{false, true} {
						width, height := 1140, nativeTrayHeight
						fileView, textView := finder.nativeView, direct.nativeView
						if preview {
							width, height = nativeContentPreviewSize(fileItem.Type)
							fileView, textView = finder.nativePopoverView, direct.nativePopoverView
						}
						fileUI, textUI := ui.NewTester(fileView, width, height), ui.NewTester(textView, width, height)
						if !reflect.DeepEqual(fileUI.Texts(), textUI.Texts()) {
							t.Fatalf("Finder paths have different content or actions from text, preview=%t: file=%q text=%q", preview, fileUI.Texts(), textUI.Texts())
						}
						for _, label := range fileUI.Texts() {
							fileBounds, _ := fileUI.Find(label)
							textBounds, _ := textUI.Find(label)
							if fileBounds != textBounds {
								t.Fatalf("Finder content layout differs from the same copied text: %q file=%+v text=%+v", label, fileBounds, textBounds)
							}
						}
						if !preview {
							card, ok := fileUI.Find(finder.nativeCardLabel(fileItem))
							if !ok || card.W != nativeCardSize || card.H != nativeCardSize {
								t.Fatalf("path card lost shared square dimensions: %+v", card)
							}
						}
						if !bytes.Equal(fileUI.Image().Pix, textUI.Image().Pix) {
							t.Fatalf("Finder paths use a different painted surface, preview=%t", preview)
						}
					}
					if fileItem.Type != textItem.Type || !reflect.DeepEqual(beforeFinder, finder.store.Records()) || !reflect.DeepEqual(beforeDirect, direct.store.Records()) {
						t.Fatal("path presentation changed source records or classification")
					}
				})
			}
		}
	}
}

func TestFinderAndTextPathsCopyTheSameTextWithSavedProtection(t *testing.T) {
	for _, finder := range []bool{false, true} {
		for _, paths := range [][]string{{"/Volumes/Offline/My File.pdf"}, {"/Volumes/Offline/My File.pdf", "/Volumes/Offline/项目说明.txt"}} {
			for _, explicitCopy := range []bool{false, true} {
				s, item := nativePathSourceFixture(t, finder, paths...)
				if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.apple.finder"}); err != nil {
					t.Fatal(err)
				}
				s.filter()
				before := s.store.Records()
				writer := &nativeVariantWriter{}
				nativeVariantCopyService(s, writer)
				width, height := nativeContentPreviewSize(item.Type)
				tt := ui.NewTester(s.nativePopoverView, width, height)
				if writer.calls != 0 || s.semanticPreview.CopyFormat != "" {
					t.Fatal("opening a path preview wrote the clipboard or selected a format")
				}
				if explicitCopy {
					if item.Type == clipmodel.TypeFilePath {
						if err := tt.Click("Copy"); err != nil {
							t.Fatal(err)
						}
					} else {
						// Plain-text previews keep their existing controls; exercise
						// the same default copy command used by card selection.
						s.chooseNative(false)
					}
				} else {
					tt.Key(0, ui.KeyEnter)
				}
				if writer.calls != 1 || !reflect.DeepEqual(writer.data, clipmodel.TextData(strings.Join(paths, "\n"))) || !reflect.DeepEqual(writer.scope, []string{"com.apple.finder"}) {
					t.Fatalf("path copy changed content or lost source protection: writes=%d data=%+v scope=%v", writer.calls, writer.data, writer.scope)
				}
				if !reflect.DeepEqual(before, s.store.Records()) || s.preview {
					t.Fatal("path copy added/edited history or retained its preview")
				}
			}
		}
	}
}

func TestFinderPathUsesExistingSanitizedRevealFailure(t *testing.T) {
	local := filepath.Join(t.TempDir(), "private-account@example.test", "salary.pdf")
	s, item := nativePathSourceFixture(t, true, local)
	writer := &nativeVariantWriter{}
	nativeVariantCopyService(s, writer)
	before := s.store.Records()
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })
	s.openNativeContent(item.ID)
	if strings.Contains(output.String(), local) || strings.Contains(output.String(), "private-account") {
		t.Fatal("path action disclosed a private path in logs")
	}
	if !strings.Contains(output.String(), "system handler could not open") || s.message != s.tr("File unavailable") {
		t.Fatal("unavailable path lost its existing sanitized error behavior")
	}
	if writer.calls != 0 || !reflect.DeepEqual(before, s.store.Records()) {
		t.Fatal("revealing a path changed clipboard or history")
	}
}
