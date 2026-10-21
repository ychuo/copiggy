// Package pasteboard owns native clipboard observations and exact
// representation transfers. It does not own protection policy or presentation.
package pasteboard

// App identifies a running application. BundleID is the stable allowlist key;
// Name is the original localized application name supplied by macOS.
type App struct {
	BundleID string
	Name     string
	PID      int
}
