package guard

import "testing"

func TestPolicyOnlyBlocksMatchingRestrictedGeneration(t *testing.T) {
	policy := Policy{Version: 42, AllowedApps: []string{"com.example.work"}}
	for _, test := range []struct {
		name    string
		version int64
		target  string
		panel   bool
		key     uint16
		flags   uint64
		want    bool
	}{
		{"restricted destination", 42, "com.example.personal", false, KeyV, ModifierCmd, true},
		{"allowed destination", 42, "com.example.work", false, KeyV, ModifierCmd, false},
		{"new clipboard does not inherit restriction", 43, "com.example.personal", false, KeyV, ModifierCmd, false},
		{"old clipboard does not inherit restriction", 41, "com.example.personal", false, KeyV, ModifierCmd, false},
		{"panel editing", 42, "com.example.personal", true, KeyV, ModifierCmd, false},
		{"unknown target", 42, "", false, KeyV, ModifierCmd, true},
		{"match style", 42, "com.example.personal", false, KeyV, ModifierCmd | ModifierAlt | ModifierShift, true},
		{"unrelated shortcut", 42, "com.example.personal", false, KeyV, ModifierCmd | ModifierCtrl, false},
		{"typing v", 42, "com.example.personal", false, KeyV, 0, false},
		{"copy", 42, "com.example.personal", false, 0x08, ModifierCmd, false},
		{"caps lock does not bypass", 42, "com.example.personal", false, KeyV, ModifierCmd | (1 << 16), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Evaluate(policy, test.version, test.target, test.panel, test.key, test.flags); got != test.want {
				t.Fatalf("Evaluate() = %v, want %v", got, test.want)
			}
		})
	}
	if Evaluate(Policy{Version: 42}, 42, "com.example.personal", false, KeyV, ModifierCmd) {
		t.Fatal("an unrestricted item must pass")
	}
}

func TestBlockedKeyUpAndRepeatsStayPaired(t *testing.T) {
	var sequence keySequence
	if !sequence.decide(true, KeyV, true) {
		t.Fatal("blocked down escaped")
	}
	if sequence.decide(false, 0x08, false) {
		t.Fatal("unrelated key-up was suppressed")
	}
	if !sequence.decide(true, KeyV, false) {
		t.Fatal("auto-repeat escaped after policy changed")
	}
	if !sequence.decide(false, KeyV, false) {
		t.Fatal("matching up escaped after modifiers changed")
	}
	if sequence.decide(false, KeyV, false) {
		t.Fatal("an unmatched up was suppressed")
	}
	if sequence.decide(true, KeyV, false) || sequence.decide(false, KeyV, true) {
		t.Fatal("an allowed key sequence was suppressed")
	}
}
