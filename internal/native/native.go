// Package native bridges the few system APIs MyGo does not expose: the
// clipboard change counter, the frontmost application, the macOS
// accessibility permission, and a synthetic paste keystroke.
//
// Everything here is plain Go: the macOS side drives Objective-C through
// purego, the Windows side calls user32 through syscall, so the app still
// builds without cgo on any machine.
package native

// App identifies the application that owned the clipboard when a record was
// captured.
type App struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	// Path is the .app bundle (macOS) or executable (Windows), when known.
	Path string `json:"path"`
	Pid  int    `json:"pid"`
	Self bool   `json:"self"`
}
