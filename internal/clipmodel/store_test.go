package clipmodel

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestEnhancementPreviewCancelAndIndependentPolicy(t *testing.T) {
	store := NewStore(10)
	source, _ := store.Capture("\r\n  first  \r\n\r\n\r\nsecond\t\r\n", "com.work.editor", false)
	if err := store.SetAllowedApps(source.ID, []string{"com.work.chat"}); err != nil {
		t.Fatal(err)
	}
	other, _ := store.Capture("current external value", "com.personal.browser", false)

	draft, err := store.Enhance(context.Background(), source.ID, CleanWhitespace)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Text != "  first\n\nsecond" || draft.ParentID != source.ID {
		t.Fatalf("unexpected draft: %+v", draft)
	}
	assertCurrent(t, store, other.ID)
	if len(store.Items()) != 2 {
		t.Fatal("preview must not create history")
	}
	store.Cancel(draft.ID)
	if _, err := store.ValidateDraft(draft.ID); !errors.Is(err, ErrDraftNotFound) {
		t.Fatalf("canceled draft remained valid: %v", err)
	}
	assertCurrent(t, store, other.ID)

	draft, err = store.Enhance(context.Background(), source.ID, CleanWhitespace)
	if err != nil {
		t.Fatal(err)
	}
	// Neither a changed preview slice nor a changed source snapshot may weaken
	// the internal draft policy.
	draft.AllowedApps[0] = "com.personal.chat"
	items := store.Items()
	items[1].AllowedApps[0] = "com.personal.chat"
	validated, err := store.ValidateDraft(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertCurrent(t, store, other.ID)
	if !reflect.DeepEqual(validated.AllowedApps, []string{"com.work.chat"}) {
		t.Fatalf("draft did not inherit an independent policy: %v", validated.AllowedApps)
	}
	if validated.ParentID != source.ID || validated.Text != "  first\n\nsecond" {
		t.Fatalf("incorrect validated transformation: %+v", validated)
	}
	original := store.Items()[1]
	if original.Text != source.Text || original.ParentID != "" || !original.Allows("com.work.chat") || original.Allows("com.personal.chat") || original.Allows("") {
		t.Fatal("enhancement changed the original")
	}
	if len(store.Items()) != 2 {
		t.Fatal("validation created a history record")
	}
}

func TestInvalidJSONAndCanceledGenerationDoNotModifyState(t *testing.T) {
	store := NewStore(10)
	source, _ := store.Capture("{invalid", "com.work.editor", false)
	before := store.Items()
	if _, err := store.Enhance(context.Background(), source.ID, FormatJSON); err == nil {
		t.Fatal("invalid JSON must return an error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Enhance(ctx, source.ID, CleanWhitespace); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if !reflect.DeepEqual(before, store.Items()) || len(store.drafts) != 0 {
		t.Fatal("failed or canceled enhancement changed history or created a draft")
	}
	assertCurrent(t, store, source.ID)
}

func TestExternalCopyClearsPolicyAndOwnRevisionEchoDoesNotDuplicate(t *testing.T) {
	store := NewStore(10)
	first, _ := store.Capture("identical text", "com.work.editor", false)
	apps := []string{" com.work.chat ", "com.work.chat", ""}
	if err := store.SetAllowedApps(first.ID, apps); err != nil {
		t.Fatal(err)
	}
	apps[0] = "com.personal.chat"
	first, err := store.Select(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.AllowedApps, []string{"com.work.chat"}) {
		t.Fatalf("policy was not copied and normalized: %v", first.AllowedApps)
	}
	if _, recorded := store.Capture(first.Text, "this-tool", true); recorded {
		t.Fatal("a verified own clipboard revision must not create history")
	}
	if len(store.Items()) != 1 {
		t.Fatal("own-write echo duplicated history")
	}
	second, recorded := store.Capture(first.Text, "com.personal.browser", false)
	if !recorded || second.ID == first.ID || len(second.AllowedApps) != 0 {
		t.Fatalf("external copy retained stale policy or was confused with echo: %+v", second)
	}
	if !second.Allows("com.personal.chat") {
		t.Fatal("fresh external content should be unrestricted")
	}
	assertCurrent(t, store, second.ID)
}

func TestBrowsingDoesNotSelectOrReorderAndEmptyCaptureClearsCurrent(t *testing.T) {
	store := NewStore(2)
	first, _ := store.Capture("First item", "com.editor", false)
	second, _ := store.Capture("Second item", "com.browser", false)
	before := store.Items()
	result := store.Search("FIRST")
	if len(result) != 1 || result[0].ID != first.ID {
		t.Fatalf("unexpected search: %+v", result)
	}
	assertCurrent(t, store, second.ID)
	if _, err := store.Select(first.ID); err != nil {
		t.Fatal(err)
	}
	assertCurrent(t, store, first.ID)
	if !reflect.DeepEqual(before, store.Items()) {
		t.Fatal("selecting history reordered or modified it")
	}
	if _, err := store.Select("missing"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("missing selection must fail: %v", err)
	}
	assertCurrent(t, store, first.ID)
	store.Capture("", "", false)
	if _, ok := store.Current(); ok {
		t.Fatal("empty or unsupported external content left stale current policy")
	}
	if !reflect.DeepEqual(before, store.Items()) {
		t.Fatal("clearing current changed history")
	}
	third, _ := store.Capture("Third item", "com.browser", false)
	if items := store.Items(); len(items) != 2 || items[0].ID != third.ID || items[1].ID != second.ID {
		t.Fatalf("bounded history did not retain newest records: %+v", items)
	}
}

func TestJSONFormattingPreservesNumbersAndRequiresExplicitAcceptance(t *testing.T) {
	store := NewStore(10)
	source, _ := store.Capture(`{"n":9007199254740993,"items":[1,true]}`, "com.editor", false)
	draft, err := store.Enhance(context.Background(), source.ID, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	expected := "{\n  \"n\": 9007199254740993,\n  \"items\": [\n    1,\n    true\n  ]\n}"
	if draft.Text != expected {
		t.Fatalf("unexpected formatted JSON:\n%s", draft.Text)
	}
	assertCurrent(t, store, source.ID)
	if len(store.Items()) != 1 {
		t.Fatal("formatting preview changed history")
	}
}

func TestValidationRejectsStaleDraftPolicyWithoutChangingState(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		before []string
		after  []string
	}{
		{
			name:  "new restriction",
			after: []string{"com.work.chat"},
		},
		{
			name:   "removed allowed app",
			before: []string{"com.work.chat", "com.work.editor"},
			after:  []string{"com.work.chat"},
		},
		{
			name:   "relaxed restriction still requires new preview",
			before: []string{"com.work.chat"},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store := NewStore(10)
			source, _ := store.Capture("original  \n", "com.editor", false)
			if err := store.SetAllowedApps(source.ID, scenario.before); err != nil {
				t.Fatal(err)
			}
			draft, err := store.Enhance(context.Background(), source.ID, CleanWhitespace)
			if err != nil {
				t.Fatal(err)
			}
			other, _ := store.Capture("current external value", "com.browser", false)
			if err := store.SetAllowedApps(source.ID, scenario.after); err != nil {
				t.Fatal(err)
			}
			before := store.Items()
			if _, err := store.ValidateDraft(draft.ID); !errors.Is(err, ErrDraftStale) {
				t.Fatalf("validation must reject stale policy before a native write: %v", err)
			}
			assertCurrent(t, store, other.ID)
			if !reflect.DeepEqual(before, store.Items()) {
				t.Fatal("rejecting a stale draft changed history")
			}
			if _, ok := store.drafts[draft.ID]; !ok {
				t.Fatal("rejecting a stale draft consumed the preview")
			}
			fresh, err := store.Enhance(context.Background(), source.ID, CleanWhitespace)
			if err != nil {
				t.Fatal(err)
			}
			validated, err := store.ValidateDraft(fresh.ID)
			if err != nil {
				t.Fatalf("regenerated draft should be valid: %v", err)
			}
			if !reflect.DeepEqual(validated.AllowedApps, scenario.after) {
				t.Fatalf("regenerated draft used stale policy: %v", validated.AllowedApps)
			}
		})
	}
}

func TestValidateDraftReturnsTrustedCopyWithoutSelecting(t *testing.T) {
	store := NewStore(10)
	source, _ := store.Capture("original  \n", "com.editor", false)
	if err := store.SetAllowedApps(source.ID, []string{"com.work.chat"}); err != nil {
		t.Fatal(err)
	}
	draft, err := store.Enhance(context.Background(), source.ID, CleanWhitespace)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := store.Capture("current external value", "com.browser", false)
	before := store.Items()
	draft.Text = "modified preview"
	draft.AllowedApps[0] = "com.personal.chat"

	validated, err := store.ValidateDraft(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if validated.Text != "original" || !reflect.DeepEqual(validated.AllowedApps, []string{"com.work.chat"}) {
		t.Fatalf("validation did not return the trusted snapshot: %+v", validated)
	}
	assertCurrent(t, store, other.ID)
	if !reflect.DeepEqual(before, store.Items()) {
		t.Fatal("validation changed history")
	}
	validated.AllowedApps[0] = "com.personal.chat"
	revalidated, err := store.ValidateDraft(draft.ID)
	if err != nil || !reflect.DeepEqual(revalidated.AllowedApps, []string{"com.work.chat"}) {
		t.Fatalf("validation exposed its internal policy slice: %+v, %v", revalidated, err)
	}
	if _, err := store.ValidateDraft("missing"); !errors.Is(err, ErrDraftNotFound) {
		t.Fatalf("missing draft validation returned %v", err)
	}
	assertCurrent(t, store, other.ID)
}

func TestValidationRejectsDraftWhoseSourceWasEvicted(t *testing.T) {
	store := NewStore(1)
	source, _ := store.Capture("original  ", "com.editor", false)
	draft, err := store.Enhance(context.Background(), source.ID, CleanWhitespace)
	if err != nil {
		t.Fatal(err)
	}
	current, _ := store.Capture("new value", "com.browser", false)
	before := store.Items()
	if _, err := store.ValidateDraft(draft.ID); !errors.Is(err, ErrDraftStale) {
		t.Fatalf("validation accepted an unverifiable source: %v", err)
	}
	assertCurrent(t, store, current.ID)
	if !reflect.DeepEqual(before, store.Items()) {
		t.Fatal("rejecting an evicted source changed history")
	}
}

func assertCurrent(t *testing.T, store *Store, id string) {
	t.Helper()
	current, ok := store.Current()
	if !ok || current.ID != id {
		t.Fatalf("current = %q, %v; want %q", current.ID, ok, id)
	}
}
