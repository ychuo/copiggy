//go:build darwin

package pasteboard

import (
	"bytes"
	"errors"
	"runtime"
	"unsafe"

	"copiggy/internal/clipmodel"
	nativeobjc "copiggy/internal/macos/internal/objc"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/transfer"
)

type nativeFilePayload struct {
	url, filenames []byte
}

func nativeCall(object uintptr, selector string, arguments ...uintptr) uintptr {
	return nativeobjc.Send(object, objc.RegisterName(selector), arguments...)
}

func nativeString(value string) uintptr {
	encoded := []byte(value)
	var address uintptr
	if len(encoded) > 0 {
		address = uintptr(unsafe.Pointer(&encoded[0]))
	}
	result := nativeCall(uintptr(objc.GetClass("NSString")), "alloc")
	result = nativeCall(result, "initWithBytes:length:encoding:", address, uintptr(len(encoded)), 4)
	runtime.KeepAlive(encoded)
	return nativeCall(result, "autorelease")
}

func nativeBytes(value uintptr) ([]byte, error) {
	if value == 0 {
		return nil, nil
	}
	length := nativeCall(value, "length")
	if length > uintptr(clipmodel.MaxClipboardBytes) {
		return nil, clipmodel.ContentError("Content exceeds 16 MiB · Not recorded")
	}
	if length == 0 {
		return []byte{}, nil
	}
	address := nativeCall(value, "bytes")
	if address == 0 {
		return nil, errors.New("native file representation is unavailable")
	}
	return bytes.Clone(unsafe.Slice(*(**byte)(unsafe.Pointer(&address)), int(length))), nil
}

// readNativeFilePayloads retains only eager file-reference payloads. It never
// asks an NSFilePromiseReceiver to create a file or reads a referenced file.
func readNativeFilePayloads() (payloads []nativeFilePayload, err error) {
	nativeobjc.WithPool(func() {
		board := nativeobjc.Send(uintptr(readerRuntime.pasteboard), readerRuntime.selectors.general)
		items := nativeCall(board, "pasteboardItems")
		count := nativeCall(items, "count")
		if count > clipmodel.MaxFileReferences {
			err = clipmodel.ContentError("Unsupported clipboard content")
			return
		}
		var total int64
		for i := uintptr(0); i < count; i++ {
			item := nativeCall(items, "objectAtIndex:", i)
			types := nativeCall(item, "types")
			has := func(name string) bool { return nativeCall(types, "containsObject:", nativeString(name)) != 0 }
			if !has("public.file-url") && !has("NSFilenamesPboardType") && !has("dev.mygo.file-list") {
				continue
			}
			payload := nativeFilePayload{}
			for name, destination := range map[string]*[]byte{"public.file-url": &payload.url, "NSFilenamesPboardType": &payload.filenames} {
				if has(name) {
					*destination, err = nativeBytes(nativeCall(item, "dataForType:", nativeString(name)))
					if err != nil {
						return
					}
					total += int64(len(*destination))
					if total > clipmodel.MaxClipboardBytes {
						err = clipmodel.ContentError("Content exceeds 16 MiB · Not recorded")
						return
					}
				}
			}
			payloads = append(payloads, payload)
		}
	})
	return payloads, err
}

func retainNativeFilePayloads(data transfer.Data, payloads []nativeFilePayload) (transfer.Data, error) {
	items := data.Items()
	next := 0
	for i, item := range items {
		if _, err := item.Read(transfer.FileList); errors.Is(err, transfer.ErrFormat) {
			continue
		} else if err != nil {
			return transfer.Data{}, err
		}
		if next == len(payloads) {
			return transfer.Data{}, ErrChanged
		}
		payload := payloads[next]
		next++
		var reps []transfer.Representation
		for _, format := range item.Formats() {
			value, err := item.Read(format)
			if err != nil {
				return transfer.Data{}, err
			}
			reps = append(reps, transfer.Bytes(format, value))
		}
		if payload.url != nil {
			reps = append(reps, transfer.Bytes(transfer.Format(clipmodel.NativeFileURLFormat), payload.url))
		}
		if payload.filenames != nil {
			reps = append(reps, transfer.Bytes(transfer.Format(clipmodel.NativeFilenamesFormat), payload.filenames))
		}
		items[i] = transfer.NewItem(reps...)
	}
	if next != len(payloads) {
		return transfer.Data{}, ErrChanged
	}
	return transfer.New(items...), nil
}
