package clipmodel

import (
	"errors"
	"fmt"
	"testing"
)

func TestPinsSurviveOrdinaryRetentionWithoutChangingCurrent(t *testing.T) {
	s := NewStore(2)
	pin, _ := s.CaptureProtected("secret", "Editor", []string{"com.editor"})
	if err := s.SetPinned(pin.ID, true); err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		s.Capture(fmt.Sprint(i), "Other", false)
	}
	before, _ := s.Current()
	items := s.Items()
	if len(items) != 3 || len(s.Pinned()) != 1 || s.Pinned()[0].ID != pin.ID {
		t.Fatal("ordinary churn evicted pin or exceeded ordinary capacity")
	}
	if err := s.SetPinned(pin.ID, false); err != nil {
		t.Fatal(err)
	}
	if len(s.Items()) != 2 || len(s.Pinned()) != 0 {
		t.Fatal("unpin did not return record to ordinary retention")
	}
	if current, _ := s.Current(); current.ID != before.ID {
		t.Fatal("pin retention changed current clipboard")
	}
}

func TestRestorePinsPreservesScopeIdentityAndDedupButNotCurrent(t *testing.T) {
	original := NewStore(1)
	item, _ := original.CaptureProtectedContent(Content{Text: "same", SourceAppID: "com.editor"}, "Editor", []string{"com.editor"})
	item.ID = "item-300"
	restored := NewStore(1)
	if err := restored.RestorePinned(item, Content{Text: item.Text}); err != nil {
		t.Fatal(err)
	}
	if _, ok := restored.Current(); ok {
		t.Fatal("startup selected a saved pin as actual clipboard")
	}
	duplicate, recorded := restored.CaptureProtected("same", "Other", []string{"com.editor"})
	if !recorded || duplicate.ID != item.ID || !duplicate.Pinned || duplicate.SourceAppID != "com.editor" {
		t.Fatal("restore lost identity, fingerprint, pin, or source")
	}
	ordinary, _ := restored.Capture("same", "Other", false)
	if ordinary.ID == item.ID || ordinary.ID != "item-301" || ordinary.Pinned {
		t.Fatal("ordinary copy collided with or unlocked saved protected pin")
	}
	if !restored.Pinned()[0].Allows("com.editor") || restored.Pinned()[0].Allows("com.other") {
		t.Fatal("restore lost source-only policy")
	}
}

func TestRestorePinsRejectsBadDataWithoutWeakeningPolicy(t *testing.T) {
	s := NewStore(2)
	item, _ := s.Capture("safe", "Editor", false)
	tests := []Item{item, item, item}
	tests[0].ID = "garbage"
	tests[1].AllowedApps = []string{" "}
	tests[2].Text = "mismatch"
	for _, invalid := range tests {
		fresh := NewStore(2)
		if err := fresh.RestorePinned(invalid, Content{Text: "safe"}); !errors.Is(err, ErrInvalidPin) {
			t.Fatalf("bad restored pin was accepted: %v", err)
		}
		if len(fresh.Items()) != 0 {
			t.Fatal("invalid restore partially mutated store")
		}
	}
}

func TestPinCountIsBounded(t *testing.T) {
	s := NewStore(2)
	for i := range MaxPinnedItems {
		item, _ := s.Capture(fmt.Sprint(i), "Editor", false)
		if err := s.SetPinned(item.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	extra, _ := s.Capture("extra", "Editor", false)
	if err := s.SetPinned(extra.ID, true); !errors.Is(err, ErrPinLimit) {
		t.Fatalf("pin limit not enforced: %v", err)
	}
	if len(s.Pinned()) != MaxPinnedItems {
		t.Fatal("failed pin changed existing pins")
	}
}
