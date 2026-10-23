//go:build darwin

package pasteboard

import (
	"bytes"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/transfer"
)

func TestNativeFilePayloadRetentionPreservesURLBytesAndItemBoundaries(t *testing.T) {
	data := transfer.New(
		transfer.NewItem(transfer.Bytes(transfer.Text, []byte("a.txt")), transfer.Bytes(transfer.FileList, []byte("file:///tmp/a.txt"))),
		transfer.NewItem(transfer.Bytes(transfer.Text, []byte("plain text"))),
		transfer.NewItem(transfer.Bytes(transfer.FileList, []byte("file:///tmp/b.txt"))),
	)
	raw := []nativeFilePayload{{url: []byte("file:///.file/id=123.456/")}, {url: []byte("file:///tmp/b.txt")}}
	saved, err := retainNativeFilePayloads(data, raw)
	if err != nil || len(saved.Items()) != 3 {
		t.Fatalf("retention failed: %v", err)
	}
	for index, want := range map[int]string{0: "file:///.file/id=123.456/", 2: "file:///tmp/b.txt"} {
		got, err := saved.Items()[index].Read(transfer.Format(clipmodel.NativeFileURLFormat))
		if err != nil || string(got) != want {
			t.Fatalf("raw native item %d changed: %q %v", index, got, err)
		}
	}
	if _, err := saved.Items()[1].Read(transfer.Format(clipmodel.NativeFileURLFormat)); err == nil {
		t.Fatal("file payload attached to unrelated text item")
	}
	if _, err := retainNativeFilePayloads(data, raw[:1]); err != ErrChanged {
		t.Fatal("mismatched file snapshot accepted")
	}
}

func TestNativeLegacyFilePayloadRetentionDoesNotAlterResolvedPaths(t *testing.T) {
	rawLegacy := []byte("bplist00original-array-payload")
	paths := []byte("file:///tmp/a.txt\r\nfile:///tmp/b%20c.txt")
	data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.FileList, paths)))
	saved, err := retainNativeFilePayloads(data, []nativeFilePayload{{filenames: rawLegacy}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := saved.Items()[0].Read(transfer.Format(clipmodel.NativeFilenamesFormat))
	if err != nil || !bytes.Equal(got, rawLegacy) {
		t.Fatal("legacy source payload was altered")
	}
	got, err = saved.Items()[0].Read(transfer.FileList)
	if err != nil || !bytes.Equal(got, paths) {
		t.Fatal("resolved path list was altered")
	}
	if _, err := data.Items()[0].Read(transfer.Format(clipmodel.NativeFilenamesFormat)); err == nil {
		t.Fatal("retention mutated input representations")
	}
}
