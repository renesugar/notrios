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
