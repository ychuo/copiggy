package nativeui

import (
	"bytes"
	"log"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentActionFailureDoesNotLogCopiedPath(t *testing.T) {
	// A missing fixture path exercises action failure without opening an app,
	// reading the real clipboard, or depending on the user's local files.
	path := filepath.Join(t.TempDir(), "private-account@example.test", "salary.pdf")
	s, item := semanticPreviewFixture(t, path)
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })
	s.openNativeContent(item.ID)
	if strings.Contains(output.String(), path) || strings.Contains(output.String(), "private-account") {
		t.Fatal("content action error disclosed the copied path in logs")
	}
	if !strings.Contains(output.String(), "system handler could not open") {
		t.Fatal("action failure did not retain a content-free diagnostic")
	}
	assertNativeCurrent(t, s, item.ID)
}
