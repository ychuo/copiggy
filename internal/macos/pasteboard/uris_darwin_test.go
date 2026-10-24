//go:build darwin

package pasteboard

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/transfer"
)

func TestURIListAliasesKeepSavedBytesAndItemBoundaries(t *testing.T) {
	first := "# Browser URL\r\nhttps://example.com/a?q=1#section\r\n"
	second := "\r\n# Another URL\r\nhttps://example.com/b\r\nhttps://example.com/c\r\n"
	data := transfer.New(
		transfer.NewItem(transfer.Bytes(transfer.Text, []byte("ordinary text"))),
		transfer.NewItem(transfer.Bytes(transfer.URIList, []byte(first)), transfer.Bytes(transfer.HTML, []byte("<b>original</b>"))),
		transfer.NewItem(transfer.Bytes(transfer.URIList, []byte(second))),
	)
	aliases, err := uriListAliases(data)
	if err != nil || len(aliases) != 2 {
		t.Fatalf("URI aliases = %+v, %v", aliases, err)
	}
	for i, want := range []struct{ raw, url string }{{first, "https://example.com/a?q=1#section"}, {second, "https://example.com/b"}} {
		if aliases[i].item != i+1 || string(aliases[i].original) != want.raw || aliases[i].url != want.url {
			t.Fatalf("alias %d changed its source or chose the wrong URL: %+v", i, aliases[i])
		}
		aliases[i].original[0] = 'X'
		original, err := data.Items()[i+1].Read(transfer.URIList)
		if err != nil || string(original) != want.raw {
			t.Fatal("alias plan mutated the saved URI list")
		}
	}
	html, err := data.Items()[1].Read(transfer.HTML)
	if err != nil || string(html) != "<b>original</b>" {
		t.Fatal("alias plan changed another representation")
	}
}

func TestURIListAliasesIgnoreCorrectOrUnsupportedLists(t *testing.T) {
	for _, raw := range []string{
		"https://example.com/a\r\n# trailing comment\r\nhttps://example.com/b",
		"# comment\r\nfile:///tmp/a",
		"# comment\r\nfile:///tmp/a\r\nfile:///tmp/b",
		"file:///tmp/a\r\nfile:///tmp/b",
		"# comment\r\nhttps://example.com/a\r\nfile:///tmp/b",
		"# comment\r\naweme://video/123\r\nfile:///tmp/b",
		"# comment\r\njavascript://route",
		"# comment\r\n[Video](aweme://video/123)",
		"# comment\r\nhttps://user:secret@example.com/a",
		"# comment\r\nhttps://example.com/a b",
		"# comment only\r\n",
	} {
		data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.URIList, []byte(raw))))
		aliases, err := uriListAliases(data)
		if err != nil || len(aliases) != 0 {
			t.Errorf("unneeded or invalid URL alias for %q: %+v, %v", raw, aliases, err)
		}
	}
}

func TestURIListAliasesPreserveExplicitAppDestinations(t *testing.T) {
	for _, raw := range []string{"aweme://video/123?q=one", "sslocal://route?url=https%3A%2F%2Fexample.com"} {
		original := "# App link\r\n" + raw + "\r\n"
		data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.URIList, []byte(original))))
		aliases, err := uriListAliases(data)
		if err != nil || len(aliases) != 1 || aliases[0].url != raw || string(aliases[0].original) != original {
			t.Fatalf("app URI alias = %+v, %v", aliases, err)
		}
	}
}

func TestURIListAliasWriteRequiresOwnershipBeforeAndAfterRepair(t *testing.T) {
	raw := "# Browser URL\r\nhttps://example.com/a\r\n"
	data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.URIList, []byte(raw))))
	failure := errors.New("alias repair unavailable")
	for _, tc := range []struct {
		name       string
		revisions  []int64
		marker     []byte
		aliasErr   error
		loseMarker bool
		wantAlias  bool
		wantErr    error
	}{
		{"owned URI", []int64{42, 42, 42}, nil, nil, false, true, nil},
		{"foreign marker before repair", []int64{42}, []byte("foreign"), nil, false, false, ErrChanged},
		{"changed revision before repair", []int64{42, 43}, nil, nil, false, false, ErrChanged},
		{"changed revision after repair", []int64{42, 42, 43}, nil, nil, false, true, ErrChanged},
		{"foreign marker after repair", []int64{42, 42}, nil, nil, true, true, ErrChanged},
		{"repair failure", []int64{42, 42}, nil, failure, false, true, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			destination := &memoryTransfer{revisions: tc.revisions, marker: tc.marker, aliasErr: tc.aliasErr}
			if tc.loseMarker {
				destination.aliasHook = func() { destination.marker = []byte("foreign") }
			}
			revision, err := writeOwned(data, []byte("owned"), destination)
			if !errors.Is(err, tc.wantErr) || (err == nil && revision != 42) || (err != nil && revision != 0) {
				t.Fatalf("write authority = %d, %v; wanted %v", revision, err, tc.wantErr)
			}
			if (len(destination.aliases) > 0) != tc.wantAlias {
				t.Fatalf("repair ran without prior ownership proof: %+v", destination.aliases)
			}
			if tc.wantAlias && (destination.aliasRevision != 42 || len(destination.aliases) != 1 || destination.aliases[0].url != "https://example.com/a") {
				t.Fatalf("repair did not target the verified generation: %+v", destination.aliases)
			}
			original, readErr := destination.data.Read(transfer.URIList)
			if readErr != nil || string(original) != raw || destination.writes != 1 || destination.flushes != 1 {
				t.Fatal("alias repair changed the original URI bytes or repeated the write transaction")
			}
		})
	}
}
