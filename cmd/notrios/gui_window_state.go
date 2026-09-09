//go:build gui

package main

import "sync"

// WindowState is the one thing the window knows about itself that the Go side
// cannot see: whether the editor is holding work that is in no store yet.
//
// It exists because closing the window is final in a way the frontend cannot
// intercept. The browser's `beforeunload` covers a reload and a closed tab; a
// Wails window closed from its own title bar or from File → Quit does not go
// through it, and on Linux the close arrives as a GTK delete-event that Wails
// turns straight into a quit. `OnBeforeClose` is the only place that question
// can be asked, and it is answered in Go, so Go has to be told.
//
// Bound in both modes, unlike NativeUIBridge, which is bound only when this
// process owns the store. That is not an exception to that rule: a -gui-only
// window edits notes too and its close button is just as final, and this
// object reaches no store, no filesystem and no network. All it can do is make
// the application ask before it closes.
//
// It carries no note title, and nothing else the person wrote. The dialog says
// that the note has unsaved changes without naming it -- the note is on the
// screen behind the dialog -- which keeps the person's content out of this
// process's window layer entirely. It also sidesteps a hazard worth recording
// for whoever writes the next dialog: on Linux Wails hands the message to
// gtk_message_dialog_new as the *format* string, so a note titled "50% done"
// would make GTK read an argument that was never passed.
type WindowState struct {
	mu      sync.Mutex
	unsaved bool
	actions *actionLog
}

// SetUnsavedChanges records what the editor is holding. Called by the frontend
// whenever that changes, and idempotent: the frontend re-sends the current
// value if the binding was not yet injected when the state first changed.
func (w *WindowState) SetUnsavedChanges(unsaved bool) {
	if w == nil {
		return
	}
	w.mu.Lock()
	changed := w.unsaved != unsaved
	w.unsaved = unsaved
	w.mu.Unlock()
	// Logged without the note it refers to, per the transcript's rule. Whether
	// the window believed there was unsaved work is exactly what one wants to
	// know afterwards if it closed when it should not have.
	if changed && w.actions != nil {
		if unsaved {
			w.actions.record("editor", "unsaved changes present")
		} else {
			w.actions.record("editor", "no unsaved changes")
		}
	}
}

// hasUnsavedChanges is what OnBeforeClose asks.
func (w *WindowState) hasUnsavedChanges() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.unsaved
}
