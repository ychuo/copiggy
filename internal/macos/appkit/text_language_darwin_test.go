//go:build darwin

package appkit

import "testing"

// These fixtures exercise an in-memory language model only. They read no
// desktop, clipboard, application, or user preference state.
func TestTextLanguageChineseVariants(t *testing.T) {
	if !initTextLanguageNative() {
		t.Fatal("local language recognizer unavailable")
	}
	for _, test := range []struct{ text, want string }{
		{"复制内容与设置", "zh-Hans"},
		{"複製內容與設定", "zh-Hant"},
		{"保存复制记录，随时查看之前的内容。", "zh-Hans"},
		{"儲存複製紀錄，隨時查看之前的內容。", "zh-Hant"},
	} {
		if got := TextLanguage(test.text); got != test.want {
			t.Errorf("TextLanguage(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}
