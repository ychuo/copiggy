package clipmodel

import (
	"context"
	"reflect"
	"sync"
	"testing"
)

func TestProtectedCaptureAndEnhancementKeepIndependentScope(t *testing.T) {
	store := NewStore(10)
	apps := []string{" com.source.editor ", "com.source.editor", ""}
	protected, recorded := store.CaptureProtected(`{"key":"value"}`, "com.source.editor", apps)
	if !recorded || protected.ID == "" {
		t.Fatal("protected copy was not captured")
	}
	apps[0] = "com.other.app"
	protected.AllowedApps[0] = "com.other.app"
	current, ok := store.Current()
	if !ok || !reflect.DeepEqual(current.AllowedApps, []string{"com.source.editor"}) {
		t.Fatalf("protected scope was not normalized and isolated: %+v", current)
	}
	if !current.Allows("com.source.editor") || current.Allows("com.other.app") || current.Allows("") {
		t.Fatal("protected item must only allow its source application")
	}

	draft, err := store.Enhance(context.Background(), protected.ID, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	assertCurrent(t, store, protected.ID)
	if !reflect.DeepEqual(draft.AllowedApps, []string{"com.source.editor"}) {
		t.Fatalf("draft lost protected scope: %+v", draft)
	}
	validated, err := store.ValidateDraft(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if validated.Text != "{\n  \"key\": \"value\"\n}" {
		t.Fatalf("unexpected enhanced content: %q", validated.Text)
	}
	if validated.ParentID != protected.ID || !reflect.DeepEqual(validated.AllowedApps, []string{"com.source.editor"}) {
		t.Fatalf("enhanced draft lost source or protected scope: %+v", validated)
	}
	assertCurrent(t, store, protected.ID)
	original := store.Items()[0]
	if original.ID != protected.ID || original.Text != `{"key":"value"}` ||
		!reflect.DeepEqual(original.AllowedApps, []string{"com.source.editor"}) {
		t.Fatalf("validating enhancement changed the protected original: %+v", original)
	}
}

func TestOrdinarySameTextCopyKeepsProtectedHistoryWithinCapacity(t *testing.T) {
	store := NewStore(2)
	store.Capture("oldest item", "com.other.app", false)
	protected, _ := store.CaptureProtected("same text", "com.source.editor", []string{"com.source.editor"})
	ordinary, recorded := store.Capture("same text", "com.source.editor", false)
	if !recorded || ordinary.ID == protected.ID || len(ordinary.AllowedApps) != 0 {
		t.Fatalf("ordinary copy did not become an independent unrestricted item: %+v", ordinary)
	}
	assertCurrent(t, store, ordinary.ID)
	items := store.Items()
	if len(items) != 2 || items[0].ID != ordinary.ID || items[1].ID != protected.ID {
		t.Fatalf("capacity handling discarded or merged the protected record: %+v", items)
	}
	if !items[1].Allows("com.source.editor") || items[1].Allows("com.other.app") {
		t.Fatal("ordinary copying relaxed the older protected record")
	}
	reselected, err := store.Select(protected.ID)
	if err != nil || len(reselected.AllowedApps) != 1 || reselected.Allows("com.other.app") {
		t.Fatalf("reselecting protected history did not restore its scope: %+v, %v", reselected, err)
	}
}

func TestProtectedCaptureRejectsMissingScopeWithoutChangingState(t *testing.T) {
	store := NewStore(1)
	first, _ := store.CaptureProtected("first", "com.source.editor", []string{"com.source.editor"})
	before := store.Items()
	for _, invalid := range []struct {
		text string
		apps []string
	}{
		{"", []string{"com.source.editor"}},
		{"text", nil},
		{"text", []string{"", " ", "\t"}},
	} {
		if _, recorded := store.CaptureProtected(invalid.text, "com.source.editor", invalid.apps); recorded {
			t.Fatal("invalid protected copy was recorded")
		}
		assertCurrent(t, store, first.ID)
		if !reflect.DeepEqual(before, store.Items()) {
			t.Fatal("rejected protected copy changed history")
		}
	}
	second, recorded := store.CaptureProtected("second", "com.second.editor", []string{"com.second.editor"})
	if !recorded {
		t.Fatal("valid protected copy was not recorded")
	}
	assertCurrent(t, store, second.ID)
	items := store.Items()
	if len(items) != 1 || items[0].ID != second.ID || !items[0].Allows("com.second.editor") ||
		items[0].Allows("com.source.editor") {
		t.Fatalf("capacity-one replacement lost its new scope: %+v", items)
	}
}

func TestProtectedCaptureNeverPublishesUnrestrictedCurrent(t *testing.T) {
	store := NewStore(2)
	store.CaptureProtected("initial", "com.source.editor", []string{"com.source.editor"})
	var readers sync.WaitGroup
	done := make(chan struct{})
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-done:
				return
			default:
				current, ok := store.Current()
				if !ok || len(current.AllowedApps) != 1 || current.Allows("com.other.app") {
					t.Errorf("concurrent reader observed unrestricted intermediate state: %+v", current)
					return
				}
			}
		}
	}()
	for range 1000 {
		store.CaptureProtected("next protected copy", "com.source.editor", []string{"com.source.editor"})
	}
	close(done)
	readers.Wait()
}
