package clipmodel

import (
	"encoding/pem"
	"strings"
	"testing"
)

func TestPrivateKeyWholePEMBlocksKeepOriginalAndScope(t *testing.T) {
	for _, kind := range []string{"PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY", "ENCRYPTED PRIVATE KEY", "DSA PRIVATE KEY", "OPENSSH PRIVATE KEY"} {
		t.Run(kind, func(t *testing.T) {
			payload := []byte(strings.Repeat("fixture", 8))
			if kind == "OPENSSH PRIVATE KEY" {
				payload = append([]byte("openssh-key-v1\x00"), payload...)
			}
			input := " \t" + string(pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: payload})) + "\n "
			analysis := Analyze(input)
			if analysis.Type != TypePrivateKey || analysis.Secret == nil || analysis.Secret.Value != strings.TrimSpace(input) {
				t.Fatalf("complete private key not recognized: %+v", analysis)
			}
			store := NewStore(1)
			item, ok := store.CaptureProtected(input, "Fixture Editor", []string{"fixture.editor"})
			if !ok || item.Type != TypePrivateKey || item.Text != input || !item.Allows("fixture.editor") || item.Allows("other.app") {
				t.Fatal("PEM recognition changed the original or protection")
			}
		})
	}
}

func TestPrivateKeyPEMHeadersRemainSupported(t *testing.T) {
	block := &pem.Block{
		Type: "RSA PRIVATE KEY",
		Headers: map[string]string{
			"Proc-Type": "4,ENCRYPTED",
			"DEK-Info":  "AES-256-CBC,00112233445566778899AABBCCDDEEFF",
		},
		Bytes: []byte(strings.Repeat("fixture", 8)),
	}
	encoded := string(pem.EncodeToMemory(block))
	for _, input := range []string{encoded, strings.ReplaceAll(encoded, "\n", "\r\n")} {
		analysis := Analyze(input)
		if analysis.Type != TypePrivateKey || analysis.Secret == nil || analysis.Secret.Value != strings.TrimSpace(input) {
			t.Fatalf("PEM headers were rejected: %+v", analysis)
		}
	}
}

func TestPrivateKeyRejectsPartialOrMultiplePEMBlocks(t *testing.T) {
	valid := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte(strings.Repeat("fixture", 8))}))
	for _, input := range []string{
		"-----BEGIN BROKEN-----\n" + valid,
		"-----BEGIN BROKEN\n" + valid,
		"-----BEGIN PRIVATE KEY-----\ninvalid\n-----END PRIVATE KEY-----\n" + valid,
		"-----BEGIN PRIVATE KEY-----\n" + valid,
		valid + valid,
		valid + "-----BEGIN BROKEN-----\n",
		valid + "-----END PRIVATE KEY-----\n",
		"Private key:\n" + valid,
		valid + "trailing content",
		strings.Replace(valid, "-----END PRIVATE KEY-----", "-----END RSA PRIVATE KEY-----", 1),
	} {
		if got := Analyze(input); got.Type != TypeText || got.Secret != nil {
			t.Errorf("partial or multiple PEM blocks accepted: %q", input)
		}
	}
}
