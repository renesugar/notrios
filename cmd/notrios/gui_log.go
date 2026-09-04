//go:build gui

package main

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// A transcript of what the interface did, for when something goes wrong.
//
// The service already logs what it does. The interface logged nothing, so a
// report that "importing did nothing" had no account of whether the dialog
// opened, whether a folder was chosen, whether the import was requested, or
// whether it failed and the message was missed. Every other desktop notes
// application keeps such a log for exactly this reason.
//
// It writes through the same standard logger the service uses, so it lands in
// the same stream, interleaved with the service's own lines in real order --
// which is the point. "Import requested" followed by a store error three lines
// later is a diagnosis; either alone is a guess.
//
// What it must never carry: note titles or bodies, search queries, tag names,
// credentials, or key material. Those are the person's content, and a
// diagnostic log is the classic place where content leaks into a bug report
// attachment. Action names, job kinds, counts, durations and outcomes are
// enough to say what happened. Filesystem paths are permitted because this log
// stays on the machine that chose them and because a failed import is usually
// a wrong folder -- but that is the boundary, and it is deliberate.
type actionLog struct {
	mu      sync.Mutex
	entries []string
	enabled bool
}

// Bounded, because a log nobody empties is a leak. The tail is what matters
// when something has just gone wrong.
const actionLogLimit = 500

func (a *actionLog) record(category, detail string) {
	category, detail = strings.TrimSpace(category), strings.TrimSpace(detail)
	if category == "" {
		category = "ui"
	}
	line := time.Now().UTC().Format(time.RFC3339) + " " + category + ": " + detail
	a.mu.Lock()
	a.entries = append(a.entries, line)
	if len(a.entries) > actionLogLimit {
		a.entries = a.entries[len(a.entries)-actionLogLimit:]
	}
	enabled := a.enabled
	a.mu.Unlock()
	if enabled {
		log.Printf("ui %s: %s", category, detail)
	}
}

// tail returns the transcript. Never nil, always a slice.
//
// A nil slice marshals to JSON null, the frontend read .length off it, and the
// dialog threw during render -- which unmounts the whole tree, so the window
// went blank with no message anywhere. It only happened when the transcript was
// empty, which is every fresh start, and it took a crash reporter to find after
// three wrong guesses. An empty collection crossing a language boundary must be
// empty, not absent.
func (a *actionLog) tail() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, len(a.entries))
	return append(out, a.entries...)
}

// LogAction records one thing the interface did.
//
// The frontend calls this; there is no filtering here beyond length, because
// deciding what is worth recording belongs where the action happens. Detail is
// truncated rather than rejected: a log line is never important enough to fail
// the operation it describes.
func (b *NativeUIBridge) LogAction(category, detail string) {
	if b == nil || b.actions == nil {
		return
	}
	if len(detail) > 400 {
		detail = detail[:400] + "…"
	}
	b.actions.record(category, detail)
}

// RecentActions returns the transcript, newest last.
//
// It is what the About dialog offers to copy alongside the build, so that a
// bug report can carry both without anyone having to find a file.
func (b *NativeUIBridge) RecentActions() []string {
	if b == nil || b.actions == nil {
		return nil
	}
	return b.actions.tail()
}

// verboseUILog reports whether interface actions are echoed to the log stream
// as they happen, rather than only kept for the transcript.
//
// It is an environment variable rather than a setting because it is a
// developer's and a support conversation's switch, not a preference: "run it
// once with NOTRIOS_UI_LOG=1 and send me the output" is the whole workflow.
func verboseUILog() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NOTRIOS_UI_LOG"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
