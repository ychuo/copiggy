//go:build darwin

package pasteboard

import (
	"bytes"
	"errors"
	"strings"

	"copiggy/internal/clipmodel"
	nativeobjc "copiggy/internal/macos/internal/objc"
	"github.com/egoist/mygo/transfer"
)

type uriListAlias struct {
	item     int
	original []byte
	url      string
}

// MyGo preserves text/uri-list as dev.mygo.uri-list, but its public.url alias
// uses the first physical line, which can be a comment or blank. Derive only
// this interoperability value; the saved list and all other formats stay exact.
func uriListAliases(data transfer.Data) ([]uriListAlias, error) {
	var aliases []uriListAlias
	for index, item := range data.Items() {
		encoded, err := item.Read(transfer.URIList)
		if errors.Is(err, transfer.ErrFormat) {
			continue
		}
		if err != nil {
			return nil, err
		}
		content, err := clipmodel.ContentFromRepresentations(clipmodel.Representations{{{Format: clipmodel.URIListFormat, Bytes: encoded}}})
		if err != nil || (content.Type != clipmodel.TypeLink && content.Type != clipmodel.TypeAppLink && content.Type != clipmodel.TypeText) {
			continue
		}
		first, _, _ := strings.Cut(content.Text, "\n")
		// Several local file references also produce ordinary multiline text.
		// Only an explicit webpage/app URI belongs in this compatibility repair.
		link := clipmodel.ParseLink(first)
		if link == nil || link.Inferred || link.Title != "" || link.URL != first {
			continue
		}
		physical, _, _ := strings.Cut(string(encoded), "\n")
		if first != strings.TrimSpace(physical) {
			aliases = append(aliases, uriListAlias{item: index, original: encoded, url: first})
		}
	}
	return aliases, nil
}

// UpdateURLAliases only edits items bound to the already verified generation.
// AppKit makes these item objects stale when ownership changes, so a race after
// the revision check fails instead of modifying a later owner's pasteboard.
func (n nativeTransfer) UpdateURLAliases(aliases []uriListAlias, revision int64) (err error) {
	nativeobjc.WithPool(func() {
		board := nativeobjc.Send(uintptr(readerRuntime.pasteboard), readerRuntime.selectors.general)
		unchanged := func() bool {
			return int64(nativeobjc.Send(board, readerRuntime.selectors.change)) == revision
		}
		if !unchanged() {
			err = ErrChanged
			return
		}
		items := nativeCall(board, "pasteboardItems")
		count := nativeCall(items, "count")
		bound := make([]uintptr, len(aliases))
		for i, alias := range aliases {
			if alias.item < 0 || uintptr(alias.item) >= count {
				err = ErrChanged
				return
			}
			bound[i] = nativeCall(items, "objectAtIndex:", uintptr(alias.item))
			original, readErr := nativeBytes(nativeCall(bound[i], "dataForType:", nativeString("dev.mygo.uri-list")))
			if readErr != nil || !bytes.Equal(original, alias.original) || !unchanged() {
				err = ErrChanged
				return
			}
		}
		for i, alias := range aliases {
			if !unchanged() || nativeCall(bound[i], "setString:forType:", nativeString(alias.url), nativeString("public.url")) == 0 {
				err = ErrChanged
				return
			}
		}
		if !unchanged() {
			err = ErrChanged
		}
	})
	return err
}
