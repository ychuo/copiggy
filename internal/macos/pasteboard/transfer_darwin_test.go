//go:build darwin

package pasteboard

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/transfer"
)

type memoryTransfer struct {
	data                        transfer.Data
	marker                      []byte
	revisions                   []int64
	writeErr, flushErr, readErr error
	writes, flushes             int
	aliases                     []uriListAlias
	aliasRevision               int64
	aliasErr                    error
	aliasHook                   func()
}

func (m *memoryTransfer) Write(data transfer.Data) error {
	m.writes++
	m.data = data
	return m.writeErr
}
func (m *memoryTransfer) Flush() error { m.flushes++; return m.flushErr }
func (m *memoryTransfer) UpdateURLAliases(aliases []uriListAlias, revision int64) error {
	m.aliases, m.aliasRevision = aliases, revision
	if m.aliasHook != nil {
		m.aliasHook()
	}
	return m.aliasErr
}
func (m *memoryTransfer) ReadFormat(format transfer.Format) ([]byte, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	if m.marker != nil {
		return m.marker, nil
	}
	return m.data.Read(format)
}
func (m *memoryTransfer) ChangeCount() int64 {
	result := m.revisions[0]
	if len(m.revisions) > 1 {
		m.revisions = m.revisions[1:]
	}
	return result
}

func TestWriteRequiresStableOwnedGeneration(t *testing.T) {
	data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, []byte{1, 2, 3})))
	for _, test := range []struct {
		name      string
		revisions []int64
		marker    []byte
		readErr   error
		wantError bool
	}{
		{"owned image", []int64{42, 42}, nil, nil, false},
		{"foreign marker", []int64{42, 42}, []byte("foreign"), nil, true},
		{"changed generation", []int64{42, 43}, nil, nil, true},
		{"unavailable marker", []int64{42}, nil, errors.New("read failed"), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			destination := &memoryTransfer{revisions: test.revisions, marker: test.marker, readErr: test.readErr}
			version, err := writeOwned(data, []byte("owned"), destination)
			if test.wantError {
				if !errors.Is(err, ErrChanged) || version != 0 {
					t.Fatalf("foreign write accepted: %d, %v", version, err)
				}
			} else if err != nil || version != 42 {
				t.Fatalf("owned generation rejected: %d, %v", version, err)
			}
			image, err := destination.data.Read(transfer.PNG)
			if err != nil || string(image) != string([]byte{1, 2, 3}) {
				t.Fatal("write changed original image representation")
			}
			if destination.writes != 1 || destination.flushes != 1 {
				t.Fatal("ownership was checked before complete write")
			}
		})
	}
}

func TestWriteFailureNeverProducesAuthority(t *testing.T) {
	data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte("same text"))))
	failure := errors.New("unavailable")
	for _, destination := range []*memoryTransfer{{writeErr: failure}, {flushErr: failure}} {
		if revision, err := writeOwned(data, []byte("owned"), destination); !errors.Is(err, failure) || revision != 0 {
			t.Fatalf("failed write produced revision %d: %v", revision, err)
		}
	}
	if _, err := writeOwned(transfer.Data{}, []byte("owned"), &memoryTransfer{}); err == nil {
		t.Fatal("empty data accepted")
	}
}

func TestWriteTokenPreservesExactSavedItems(t *testing.T) {
	data := transfer.New(
		transfer.NewItem(transfer.Bytes(transfer.Text, []byte("  unchanged\n")), transfer.Bytes(transfer.HTML, []byte("<b>unchanged</b>"))),
		transfer.NewItem(transfer.Bytes(transfer.PNG, []byte{1, 2, 3})),
	)
	marked, err := withWriteToken(data, []byte("new ownership"))
	if err != nil {
		t.Fatal(err)
	}
	if len(marked.Items()) != len(data.Items()) {
		t.Fatal("ownership marker changed clipboard item boundaries")
	}
	for index, item := range data.Items() {
		for _, format := range item.Formats() {
			before, err := item.Read(format)
			if err != nil {
				t.Fatal(err)
			}
			after, err := marked.Items()[index].Read(format)
			if err != nil || string(before) != string(after) {
				t.Fatalf("write changed item %d representation %s", index, format)
			}
		}
	}
	if _, err := data.Read(writeToken); err == nil {
		t.Fatal("writing mutated saved representations with a temporary ownership marker")
	}
}
