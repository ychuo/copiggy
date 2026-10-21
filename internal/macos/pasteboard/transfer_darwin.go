//go:build darwin

package pasteboard

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

var ErrChanged = errors.New("clipboard changed during write")

const writeToken = transfer.Format("dev.copiggy.write-token")

// Read materializes the supported saved representations on MyGo's main thread.
func (r *Reader) Read() (transfer.Data, error) {
	return mygo.Clipboard.Read(transfer.Text, transfer.HTML, transfer.PNG)
}

// Write returns a generation only after an unguessable write marker proves
// ownership. The caller may bind a policy to that generation; later clipboard
// changes cannot inherit it. Call on MyGo's main thread.
func (r *Reader) Write(data transfer.Data) (int64, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return 0, err
	}
	return writeOwned(data, []byte(hex.EncodeToString(token[:])), nativeTransfer{r})
}

type transferWriter interface {
	Write(transfer.Data) error
	Flush() error
	ReadFormat(transfer.Format) ([]byte, error)
	ChangeCount() int64
}

type nativeTransfer struct{ *Reader }

func (nativeTransfer) Write(data transfer.Data) error { return mygo.Clipboard.Write(data) }
func (nativeTransfer) Flush() error                   { return mygo.Clipboard.Flush() }
func (nativeTransfer) ReadFormat(format transfer.Format) ([]byte, error) {
	return mygo.Clipboard.ReadFormat(format)
}

func writeOwned(data transfer.Data, marker []byte, destination transferWriter) (int64, error) {
	marked, err := withWriteToken(data, marker)
	if err != nil {
		return 0, err
	}
	if err = destination.Write(marked); err != nil {
		return 0, err
	}
	if err = destination.Flush(); err != nil {
		return 0, err
	}
	revision := destination.ChangeCount()
	got, err := destination.ReadFormat(writeToken)
	if err != nil || string(got) != string(marker) || revision != destination.ChangeCount() {
		return 0, ErrChanged
	}
	return revision, nil
}

func withWriteToken(data transfer.Data, marker []byte) (transfer.Data, error) {
	items := data.Items()
	if len(items) == 0 {
		return transfer.Data{}, errors.New("clipboard content has no representations")
	}
	var representations []transfer.Representation
	for _, format := range items[0].Formats() {
		if format == writeToken {
			continue
		}
		value, err := items[0].Read(format)
		if err != nil {
			return transfer.Data{}, err
		}
		representations = append(representations, transfer.Bytes(format, value))
	}
	representations = append(representations, transfer.Bytes(writeToken, marker))
	items[0] = transfer.NewItem(representations...)
	return transfer.New(items...), nil
}
