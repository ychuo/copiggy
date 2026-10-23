//go:build darwin

package appkit

import (
	"runtime"
	"sync"
	"unsafe"

	nativeobjc "copiggy/internal/macos/internal/objc"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var textLanguageNative struct {
	once        sync.Once
	available   bool
	doubleValue func(uintptr, objc.SEL) float64
}

func initTextLanguageNative() bool {
	textLanguageNative.once.Do(func() {
		// A missing optional framework leaves the deterministic font fallback in
		// place. Never include clipboard text in errors or diagnostic output.
		defer func() {
			if recover() != nil {
				textLanguageNative.available = false
			}
		}()
		if nativeobjc.Init() != nil {
			return
		}
		if _, err := purego.Dlopen("/System/Library/Frameworks/NaturalLanguage.framework/NaturalLanguage", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			return
		}
		if objc.GetClass("NLLanguageRecognizer") == 0 {
			return
		}
		purego.RegisterFunc(&textLanguageNative.doubleValue, nativeobjc.MessageSend())
		textLanguageNative.available = true
	})
	return textLanguageNative.available
}

// TextLanguage returns a bounded, on-device CJK language hint for typography:
// ja, zh-Hans, zh-Hant, or und when the text is ambiguous or unsupported. It
// does not inspect the desktop, clipboard, app locale, or system preferences.
// Callers choose a stable fallback for und and cache results outside rendering.
func TextLanguage(text string) (language string) {
	sample, hint, han := textLanguageSample(text)
	if hint != "" {
		return hint
	}
	language = "und"
	// A single shared ideograph or short name has too little linguistic context.
	if han < 4 || !initTextLanguageNative() {
		return language
	}
	defer func() {
		if recover() != nil {
			language = "und"
		}
	}()
	nativeobjc.WithPool(func() {
		recognizer := nativeobjc.Send(nativeobjc.Send(uintptr(objc.GetClass("NLLanguageRecognizer")), objc.RegisterName("alloc")), objc.RegisterName("init"))
		if recognizer == 0 {
			return
		}
		defer nativeobjc.Send(recognizer, objc.RegisterName("release"))
		bytes := []byte(sample)
		str := nativeobjc.Send(nativeobjc.Send(uintptr(objc.GetClass("NSString")), objc.RegisterName("alloc")), objc.RegisterName("initWithBytes:length:encoding:"), uintptr(unsafe.Pointer(unsafe.SliceData(bytes))), uintptr(len(bytes)), 4)
		runtime.KeepAlive(bytes)
		if str == 0 {
			return
		}
		defer nativeobjc.Send(str, objc.RegisterName("release"))
		nativeobjc.Send(recognizer, objc.RegisterName("processString:"), str)
		tag := nativeobjc.Send(recognizer, objc.RegisterName("dominantLanguage"))
		if tag == 0 {
			return
		}
		hypotheses := nativeobjc.Send(recognizer, objc.RegisterName("languageHypothesesWithMaximum:"), 1)
		probability := nativeobjc.Send(hypotheses, objc.RegisterName("objectForKey:"), tag)
		if probability == 0 {
			return
		}
		language = supportedTextLanguage(nativeobjc.String(tag), textLanguageNative.doubleValue(probability, objc.RegisterName("doubleValue")))
	})
	return language
}
