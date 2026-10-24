package clipmodel

import (
	"strings"
	"testing"
)

func TestBearerWrappersRequireRecognizedTokensAndKeepOriginal(t *testing.T) {
	for _, token := range []string{testJWT(), "ghp_" + strings.Repeat("a", 36), "sk-" + strings.Repeat("b", 32)} {
		for _, prefix := range []string{"Bearer ", "bearer\t", "Authorization: Bearer ", "authorization:\tBEARER  "} {
			input := prefix + token
			analysis := Analyze(input)
			if analysis.Type != TypeToken || analysis.Secret == nil || analysis.Secret.Value != token || analysis.Secret.Kind != Analyze(token).Secret.Kind {
				t.Fatalf("wrapped credential not recognized: %+v", analysis)
			}
			store := NewStore(1)
			item, ok := store.CaptureProtected(input, "Fixture Editor", []string{"fixture.editor"})
			if !ok || item.Type != TypeToken || item.Text != input || !item.Allows("fixture.editor") || item.Allows("other.app") {
				t.Fatal("wrapper recognition changed the original or protection")
			}
		}
	}
}

func TestBearerWrappersDoNotExtractArbitraryCredentials(t *testing.T) {
	token := "ghp_" + strings.Repeat("a", 36)
	for _, input := range []string{
		"Bearer", "Bearer random-string", "Bearer " + strings.Repeat("a", 64),
		"Authorization: Basic " + token, "Authorization: " + token,
		"Bearer \"" + token + "\"", "Bearer " + token + ", " + token,
		"Bearer " + token + " more", "Use Bearer " + token,
		"Authorization: Bearer " + token + "\nX-Extra: value", "Bearer\n" + token,
		"Bearer Bearer " + token,
	} {
		if got := Analyze(input); got.Type == TypeToken || got.Secret != nil {
			t.Errorf("malformed credential wrapper accepted: %q", input)
		}
	}
}
