package clipmodel

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRetainSharesByteBudgetWithPinsAndKeepsAgeBoundary(t *testing.T) {
	now := time.Now()
	payload := make([]byte, 16<<20)
	var records []Record
	for i := 0; i < 5; i++ {
		records = append(records, Record{Item: Item{ID: fmt.Sprint(i), CreatedAt: now, Pinned: i > 0}, Data: Representations{{{Format: "application/test", Bytes: payload}}}})
	}
	retained, err := Retain(records, 30, now)
	if err != nil || len(retained) != 4 {
		t.Fatalf("shared byte budget: %d %v", len(retained), err)
	}
	for _, r := range retained {
		if !r.Item.Pinned {
			t.Fatal("ordinary record displaced pinned bytes")
		}
	}
	records[0].Item.Pinned = true
	if _, err = Retain(records, 30, now); !errors.Is(err, ErrPinLimit) {
		t.Fatal("pins exceeded shared byte budget")
	}
	boundary := now.AddDate(0, 0, -7)
	records = []Record{{Item: Item{ID: "boundary", CreatedAt: boundary}}, {Item: Item{ID: "fresh", CreatedAt: boundary.Add(time.Nanosecond)}}, {Item: Item{ID: "old-pin", CreatedAt: boundary.AddDate(0, 0, -100), Pinned: true}}}
	retained, err = Retain(records, 7, now)
	if err != nil || len(retained) != 2 || retained[0].Item.ID != "fresh" || retained[1].Item.ID != "old-pin" {
		t.Fatal("age cutoff or pin exemption changed")
	}
	retained, err = Retain(records, 0, now)
	if err != nil || len(retained) != 3 {
		t.Fatal("never-by-age preference expired records")
	}
}

func TestRepresentationsClonePreservesItemAndFormatBoundaries(t *testing.T) {
	original := Representations{{{Format: HTMLFormat, Bytes: []byte("<b>a</b>")}, {Format: TextFormat, Bytes: []byte("a")}}, {{Format: TextFormat, Bytes: []byte("b")}}}
	copy := original.Clone()
	copy[0][0].Bytes[0] = 'X'
	copy[0][0].Format = "changed"
	if original[0][0].Format != HTMLFormat || string(original[0][0].Bytes) != "<b>a</b>" {
		t.Fatal("clone shared writable representation")
	}
	text, err := original.Read(TextFormat)
	if err != nil || string(text) != "a\nb" {
		t.Fatal("multi-item plain text semantics changed")
	}
	html, err := original.Read(HTMLFormat)
	if err != nil || string(html) != "<b>a</b>" {
		t.Fatal("HTML lookup changed")
	}
}
