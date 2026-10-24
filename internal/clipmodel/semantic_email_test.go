package clipmodel

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEmailRecognizesOneWrappedOrMailtoRecipient(t *testing.T) {
	for _, tc := range []struct{ input, address, local, domain string }{
		{"Name <hello@example.com>", "hello@example.com", "hello", "example.com"},
		{"<hello@example.com>", "hello@example.com", "hello", "example.com"},
		{"  Jane Doe <First.Last+tag@EXAMPLE.com> \n", "First.Last+tag@EXAMPLE.com", "First.Last+tag", "example.com"},
		{`"Doe, Jane" <jane@example.com>`, "jane@example.com", "jane", "example.com"},
		{"mailto:hello@example.com", "hello@example.com", "hello", "example.com"},
		{"MAILTO:Alex+tag@EXAMPLE.com", "Alex+tag@EXAMPLE.com", "Alex+tag", "example.com"},
		{"mailto:Alex%2Btag%40EXAMPLE.com", "Alex+tag@EXAMPLE.com", "Alex+tag", "example.com"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			analysis := Analyze(tc.input)
			email := analysis.Email
			if analysis.Type != TypeEmail || email == nil || email.Address != tc.address || email.Local != tc.local || email.Domain != tc.domain {
				t.Fatalf("email = %+v, want %q", email, tc.address)
			}
			store := NewStore(1)
			allowed := []string{"com.example.editor"}
			item, ok := store.CaptureProtectedContent(Content{Text: tc.input}, "Fixture Editor", allowed)
			if !ok || item.Type != TypeEmail || item.Text != tc.input || !reflect.DeepEqual(item.AllowedApps, allowed) {
				t.Fatal("email recognition rewrote original text or protection")
			}
		})
	}
}

func TestEmailRejectsHeadersMultipleRecipientsAndMalformedWrappers(t *testing.T) {
	for _, input := range []string{
		"hello@example.com,other@example.com", "Name <hello@example.com>, Other <other@example.com>",
		"Name <hello@example.com>;other@example.com", "Name <<hello@example.com>>", "Name <hello@example.com", "hello@example.com>",
		"Name <hello@example.com> then send", "Send to hello@example.com", "To: Name <hello@example.com>",
		"Name <hello@example.com>\r\nBcc:other@example.com", "Name\n<hello@example.com>",
		"<hello@localhost>", "Name <hello@example..com>", "Name <hello@-example.com>",
		"mailto:", "mailto://hello@example.com", "mailto:hello@example.com,other@example.com", "mailto:hello@example.com;other@example.com",
		"mailto:hello@example.com?subject=hello", "mailto:hello@example.com?", "mailto:hello@example.com#fragment", "mailto:hello@example.com#",
		"mailto:hello@example.com%3Fsubject%3Dhello", "mailto:hello@example.com%23fragment",
		"mailto:hello@example.com%0d%0aBcc%3Aother@example.com", "mailto:hello@example.com%00",
		"mailto:hello@example.com%2Cother@example.com", "mailto:hello@example.com%3Bother@example.com", "mailto:%3Chello@example.com%3E",
		"mailto:Name%20%3Chello@example.com%3E", "mailto:hello%zz@example.com", "mailto: hello@example.com",
		"Name <" + strings.Repeat("a", 65) + "@example.com>",
	} {
		t.Run(input, func(t *testing.T) {
			if got := Analyze(input); got.Type != TypeText || got.Email != nil {
				t.Fatalf("unsafe or malformed email recognized: %+v", got)
			}
		})
	}
}

func TestEmailWrapperRestoreRetainsExactRepresentationAndScope(t *testing.T) {
	original := "  Name <hello@example.com> \n"
	data := Representations{{{Format: TextFormat, Bytes: []byte(original)}, {Format: HTMLFormat, Bytes: []byte("<b>Name &lt;hello@example.com&gt;</b>")}}}
	before := data.Clone()
	saved := Item{ID: "item-7", Type: TypeText, Text: original, SourceApp: "Editor", SourceAppID: "com.example.editor", AllowedApps: []string{"com.example.editor"}, CreatedAt: time.Now(), Pinned: true}
	model, err := RestoreRecords([]Record{{Item: saved, Data: data}})
	if err != nil {
		t.Fatal(err)
	}
	item := model.Items()[0]
	if item.Type != TypeEmail || item.ID != saved.ID || item.Text != original || !reflect.DeepEqual(item.AllowedApps, saved.AllowedApps) || !reflect.DeepEqual(data, before) {
		t.Fatal("email recognition changed saved representations or scope")
	}
	if _, selected := model.Current(); selected {
		t.Fatal("email reclassification acquired clipboard authority")
	}
}
