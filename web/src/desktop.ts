import { useEffect, useState } from 'react';
// Desktop-shell integration.
//
// The application runs in two places: a browser tab against the loopback
// service, and a Wails webview. Almost everything behaves identically, but
// following a link out of the application does not.
//
// In a browser, `target="_blank"` opens a new tab and there is nothing to do.
// In the Wails v2 webview on Linux there is: WebKitGTK raises its `create`
// signal for a `_blank` click, Wails connects no handler for it, and the
// default returns NULL — so the click is silently swallowed and the user has to
// copy the URL out by hand. Wails exposes `BrowserOpenURL` for exactly this,
// which hands the URL to the system browser.

/** The slice of the Wails runtime this module uses. */
interface DesktopRuntime {
  BrowserOpenURL?: (url: string) => void;
}

/**
 * Returns the Wails runtime when the application is running inside the desktop
 * shell.
 *
 * This feature-detects the one function it needs rather than sniffing the user
 * agent: a WebKit webview and a Safari tab report the same thing, and the
 * question here is not "which engine" but "is there a host that can open a
 * browser for us".
 */
function desktopRuntime(): DesktopRuntime | null {
  if (typeof window === 'undefined') return null;
  const candidate = (window as unknown as { runtime?: DesktopRuntime }).runtime;
  if (!candidate || typeof candidate.BrowserOpenURL !== 'function') return null;
  return candidate;
}

/** Reports whether the application is running in the desktop shell. */
export function isDesktopShell(): boolean {
  return desktopRuntime() !== null;
}

/**
 * The schemes handed to the system browser.
 *
 * Deliberately the same set the preview sanitizer already keeps on an anchor
 * (`normalizePreviewHTML` strips every other scheme's href). This function
 * changes how an already-permitted link is followed; it must never widen what
 * is followable. Anything else — including `notrios://`, `document://`, and
 * `resource://` — is handled inside the application and must not leave it.
 */
export function isExternalLink(href: string): boolean {
  const value = href.trim().toLowerCase();
  return value.startsWith('http://') || value.startsWith('https://') || value.startsWith('mailto:');
}

/**
 * Opens an external link in the user's browser when running on the desktop.
 *
 * Returns true when the desktop shell took the link, so the caller knows to
 * suppress the default. Returns false in a browser tab, where the anchor's
 * own `target="_blank"` already does the right thing and intercepting would
 * only replace working behaviour with our own.
 */
export function openExternalURL(href: string): boolean {
  if (!isExternalLink(href)) return false;
  const runtime = desktopRuntime();
  if (!runtime?.BrowserOpenURL) return false;
  runtime.BrowserOpenURL(href);
  return true;
}

/**
 * Returns true once the native bridge exists, re-rendering when it appears.
 *
 * The bridge is not there when the page loads. Wails injects `window.go` after
 * the webview starts, so a frontend that reads it once at mount can mount
 * first and conclude it is running in a browser -- and never look again. The
 * cost of that is not theoretical: the Import/Export control stays disabled in
 * the desktop application, and About reports "Mode: browser" while running as
 * a desktop program. Both were observed, intermittently, which is the worst
 * way for a wrong assumption to present itself.
 *
 * Polling rather than an event, because the injection is not announced by one
 * the frontend can rely on. It stops as soon as the bridge appears, and gives
 * up after a few seconds: a browser genuinely has no bridge, and an interval
 * that ran forever would be a timer burning in every tab.
 */
export function useNativeBridgeReady(): boolean {
  const [ready, setReady] = useState(() => nativeBridge() !== undefined);
  useEffect(() => {
    if (ready) return undefined;
    const startedAt = Date.now();
    const timer = window.setInterval(() => {
      if (nativeBridge() !== undefined) {
        setReady(true);
        window.clearInterval(timer);
      } else if (Date.now() - startedAt > 5000) {
        window.clearInterval(timer);
      }
    }, 150);
    return () => window.clearInterval(timer);
  }, [ready]);
  return ready;
}

/** The bound bridge object, or undefined in a browser. */
export function nativeBridge(): Record<string, unknown> | undefined {
  return (window as Window & { go?: { main?: { NativeUIBridge?: Record<string, unknown> } } })
    .go?.main?.NativeUIBridge;
}

/**
 * The window-state binding.
 *
 * Bound in both desktop modes, unlike `NativeUIBridge` -- see
 * `cmd/notrios/gui_window_state.go` for why that is not a hole in that
 * object's access control. It carries one boolean and no note content.
 */
interface WindowStateBridge {
  SetUnsavedChanges?: (unsaved: boolean) => Promise<void>;
}

function windowStateBridge(): WindowStateBridge | undefined {
  // The poller below can outlive the document it started in: it waits several
  // seconds for Wails to inject `window.go`, and a page torn down inside that
  // window leaves a tick with no `window` to read. Reaching for it then throws
  // a ReferenceError from a timer callback, where nothing is waiting to catch
  // it. Found as an intermittent CI failure -- the same commit passed one run
  // and failed the next, which is what a teardown race looks like.
  if (typeof window === 'undefined') return undefined;
  return (window as Window & { go?: { main?: { WindowState?: WindowStateBridge } } }).go?.main?.WindowState;
}

function stopUnsavedTimer(): void {
  if (unsavedTimer === undefined) return;
  // The global rather than `window.clearInterval`, so that stopping works in
  // the one situation that most needs it: the window is already gone.
  clearInterval(unsavedTimer);
  unsavedTimer = undefined;
}

let pendingUnsaved: boolean | null = null;
let unsavedTimer: number | undefined;

function deliverUnsavedChanges(): boolean {
  const bridge = windowStateBridge();
  if (pendingUnsaved === null || typeof bridge?.SetUnsavedChanges !== 'function') return false;
  const value = pendingUnsaved;
  pendingUnsaved = null;
  // Best effort. A report that fails must not interrupt typing, and the next
  // change re-sends: the value is the current state rather than an increment,
  // so a lost one is corrected by the one after it.
  void Promise.resolve(bridge.SetUnsavedChanges(value)).catch(() => {});
  return true;
}

/**
 * Tells the desktop shell whether the editor is holding unsaved work, so that
 * closing the window can ask before discarding it.
 *
 * The window's own close button does not go through `beforeunload`; the shell
 * asks instead, in Go, and can only do that if it has been told. Retried until
 * the binding appears for the same reason `useNativeBridgeReady` polls: Wails
 * injects `window.go` after the webview starts, and a report sent once at the
 * moment the state changed would be dropped if it arrived first. It gives up
 * after a few seconds, because in a browser there is nothing to tell.
 */
export function reportUnsavedChanges(unsaved: boolean): void {
  pendingUnsaved = unsaved;
  if (deliverUnsavedChanges()) return;
  if (typeof window === 'undefined' || unsavedTimer !== undefined) return;
  const startedAt = Date.now();
  unsavedTimer = window.setInterval(() => {
    if (typeof window === 'undefined' || deliverUnsavedChanges() || Date.now() - startedAt > 5000) {
      stopUnsavedTimer();
    }
  }, 150);
}
