// About Notrios: which build is this, in a form somebody can paste.
//
// "0.7.0" does not identify a build between releases, and the first useful
// question about any bug report is which build the person had. The revision,
// the build time and whether the tree was modified answer it, and the Copy
// button exists so that answering costs one click rather than a transcription.
//
// The build fields come from the native bridge rather than from the HTTP API,
// because they describe *this application*. In browser mode the window may be
// showing a service on another machine whose binary it knows nothing about, so
// those lines say so instead of guessing.
import { useCallback, useEffect, useRef, useState } from 'react';
import type { StatusResponse } from '../api';

interface BuildInfo {
  version: string;
  revision: string;
  built_at: string;
  modified: boolean;
  go_version: string;
  platform: string;
  tags: string;
}

interface AboutBridge {
  About(): Promise<BuildInfo>;
  // Optional on purpose. Every method here is bound at runtime by the desktop
  // shell, so the frontend must not assume that a bridge which offers one
  // offers all of them: an older shell, or one built before this method
  // existed, would otherwise throw inside an effect and take the dialog down
  // instead of showing a build report without its activity.
  RecentActions?: () => Promise<string[]>;
}

function aboutBridge(): AboutBridge | undefined {
  const bound = (window as Window & { go?: { main?: { NativeUIBridge?: Partial<AboutBridge> } } })
    .go?.main?.NativeUIBridge;
  return bound?.About ? (bound as AboutBridge) : undefined;
}

/** The report, as one block of text: what Copy puts on the clipboard. */
export function describeBuild(
  status: StatusResponse | null,
  build: BuildInfo | null,
  actions: string[] | null = [],
): string {
  const lines = [
    `Notrios ${build?.version ?? status?.version ?? 'unknown'}`,
    `Schema: ${status?.database_info?.schema_version ?? 'unknown'}`,
  ];
  if (build) {
    lines.push(
      `Revision: ${build.revision || 'unknown'}${build.modified ? ' (modified)' : ''}`,
      `Built: ${build.built_at || 'unknown'}`,
      `Go: ${build.go_version}`,
      `Platform: ${build.platform}`,
      `Tags: ${build.tags || 'none'}`,
      'Mode: desktop application',
    );
  } else {
    lines.push('Mode: browser (build details come from the desktop application)');
  }
  if (actions && actions.length > 0) {
    // Appended rather than offered as a second button, so that what is copied
    // is exactly what the dialog shows. A Copy that quietly carries more than
    // the page displays is how private things end up in bug reports.
    lines.push('', 'Recent activity:', ...actions);
  }
  return lines.join('\n');
}

/**
 * Copies text, and reports whether it worked.
 *
 * The Clipboard API needs a secure context. http://127.0.0.1 is one, so this
 * normally takes the first path; the fallback is kept because a copy that
 * silently does nothing is worse than one that says it failed, and because a
 * profile served over plain HTTP from another host would otherwise lose the
 * button's only function without explanation.
 */
async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // Fall through: a rejected write is not a reason to give up silently.
  }
  try {
    const staging = document.createElement('textarea');
    staging.value = text;
    staging.setAttribute('readonly', '');
    staging.style.position = 'fixed';
    staging.style.opacity = '0';
    document.body.appendChild(staging);
    staging.select();
    const copied = document.execCommand('copy');
    document.body.removeChild(staging);
    return copied;
  } catch {
    return false;
  }
}

export function AboutDialog({ status, onClose }: { status: StatusResponse | null; onClose: () => void }) {
  const [build, setBuild] = useState<BuildInfo | null>(null);
  const [actions, setActions] = useState<string[]>([]);
  const [copied, setCopied] = useState('');
  // Focused on open so that the one thing a person came here to do takes a
  // single key. An automated desktop run relies on the same placement.
  const copyRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const bridge = aboutBridge();
    if (bridge) {
      void bridge.About().then(setBuild).catch(() => setBuild(null));
      void bridge.RecentActions?.().then((list) => setActions(list ?? [])).catch(() => setActions([]));
    }
    copyRef.current?.focus();
  }, []);
  // Escape closes, which is what a person expects of any dialog and what this
  // one previously did not do. Bound on the document because focus may be
  // anywhere inside by the time it is pressed.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  const report = describeBuild(status, build, actions);
  const onCopy = useCallback(async () => {
    setCopied(await copyText(report) ? 'Copied to the clipboard.' : 'This browser would not allow the copy.');
  }, [report]);

  return (
    <div className="sync-center-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section className="about-dialog" role="dialog" aria-modal="true" aria-labelledby="about-title"
        data-testid="about-dialog">
        <h2 id="about-title">About Notrios</h2>
        <pre className="about-report" data-testid="about-report">{report}</pre>
        <div className="row-actions">
          <button ref={copyRef} type="button" className="primary-button" data-testid="about-copy"
            onClick={() => void onCopy()}>Copy</button>
          <button type="button" data-testid="about-close" onClick={onClose}>OK</button>
        </div>
        {copied ? <p className="success-line" data-testid="about-copied">{copied}</p> : null}
      </section>
    </div>
  );
}
