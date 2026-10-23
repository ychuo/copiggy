package clipmodel

// AnalyzeItem uses the context captured with the original bytes. Display,
// conversion and explicit actions must not reinterpret a national number using
// a subsequently changed region preference or UI language.
func AnalyzeItem(item Item) Analysis {
	if item.Type == TypeImage {
		return Analysis{Type: item.Type}
	}
	return AnalyzeInRegion(item.Text, item.RecognitionRegion)
}
