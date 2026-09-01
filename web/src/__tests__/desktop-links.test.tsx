import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { isDesktopShell, isExternalLink, openExternalURL } from '../desktop';
import { PreviewPane } from '../components/PreviewPane';

/** Installs a fake Wails runtime and returns the calls it received. */
function withDesktopShell(): { calls: string[]; restore: () => void } {
  const calls: string[] = [];
  const original = (window as unknown as { runtime?: unknown }).runtime;
  (window as unknown as { runtime?: unknown }).runtime = {
    BrowserOpenURL: (url: string) => calls.push(url),
  };
  return {
    calls,
    restore: () => {
      (window as unknown as { runtime?: unknown }).runtime = original;
    },
  };
}

afterEach(() => {
  delete (window as unknown as { runtime?: unknown }).runtime;
  vi.restoreAllMocks();
});

describe('isExternalLink', () => {
  it('accepts exactly the schemes the preview sanitizer keeps on an anchor', () => {
    for (const href of [
      'http://example.com',
      'https://example.com/path?q=1',
      'HTTPS://EXAMPLE.COM',
      '  https://example.com  ',
      'mailto:someone@example.com',
    ]) {
      expect(isExternalLink(href), href).toBe(true);
    }
  });

  it('refuses in-app and dangerous schemes', () => {
    // These are handled inside the application and must never be handed to a
    // system browser, and the dangerous ones must never be followed at all.
    for (const href of [
      'notrios://databases/db_a/documents/doc_b',
      'document://doc_x',
      'resource://res_x',
      'javascript:alert(1)',
      'data:text/html,<script>alert(1)</script>',
      'file:///etc/passwd',
      'vbscript:msgbox(1)',
      '#heading',
      '',
      '   ',
    ]) {
      expect(isExternalLink(href), href).toBe(false);
    }
  });

  it('is not fooled by a scheme appearing later in the string', () => {
    expect(isExternalLink('notrios://x/https://evil.example')).toBe(false);
    expect(isExternalLink('javascript:void(https://example.com)')).toBe(false);
  });
});

describe('isDesktopShell', () => {
  it('is false in a browser tab', () => {
    expect(isDesktopShell()).toBe(false);
  });

  it('is true when the host exposes BrowserOpenURL', () => {
    const shell = withDesktopShell();
    expect(isDesktopShell()).toBe(true);
    shell.restore();
  });

  it('is false when a runtime exists without the function we need', () => {
    // Feature detection, not the mere presence of a global: another host could
    // define window.runtime for something else entirely.
    (window as unknown as { runtime?: unknown }).runtime = { EventsOn: () => {} };
    expect(isDesktopShell()).toBe(false);
  });
});

describe('openExternalURL', () => {
  it('hands a remote URL to the host and reports that it took it', () => {
    const shell = withDesktopShell();
    expect(openExternalURL('https://example.com/x')).toBe(true);
    expect(shell.calls).toEqual(['https://example.com/x']);
    shell.restore();
  });

  it('does nothing in a browser tab, leaving target=_blank to work', () => {
    expect(openExternalURL('https://example.com/x')).toBe(false);
  });

  it('refuses to hand an in-app or dangerous scheme to the browser', () => {
    const shell = withDesktopShell();
    for (const href of [
      'notrios://databases/db_a/documents/doc_b',
      'javascript:alert(1)',
      'file:///etc/passwd',
    ]) {
      expect(openExternalURL(href), href).toBe(false);
    }
    expect(shell.calls).toEqual([]);
    shell.restore();
  });
});

describe('preview clicks in the desktop shell', () => {
  const noop = () => {};

  function renderPreview(body: string) {
    return render(
      <PreviewPane
        body={body}
        themeBase="light"
        onOpenDocument={noop}
        onOpenStableLink={noop}
        onError={noop}
      />,
    );
  }

  it('opens a remote link in the system browser and does not navigate the webview', async () => {
    const shell = withDesktopShell();
    const { container } = renderPreview('[remote](https://example.com/page)');

    const anchor = container.querySelector('a[href^="https://"]');
    expect(anchor, 'the preview did not render a remote anchor').not.toBeNull();

    // Observe the event after the handler has run. Suppressing the default is
    // the half that matters most: a webview that followed the link in place
    // would replace the running application with a website, which is worse
    // than today's silent no-op.
    let defaultPrevented: boolean | null = null;
    document.addEventListener(
      'click',
      (event) => {
        defaultPrevented = event.defaultPrevented;
      },
      { once: true },
    );

    await userEvent.click(anchor as Element);
    expect(shell.calls).toEqual(['https://example.com/page']);
    expect(defaultPrevented, 'the click default was not suppressed').toBe(true);
    shell.restore();
  });

  it('leaves the browser tab path alone', async () => {
    const { container } = renderPreview('[remote](https://example.com/page)');
    const anchor = container.querySelector('a[href^="https://"]') as HTMLAnchorElement | null;
    expect(anchor).not.toBeNull();
    // The anchor keeps the attributes the browser needs to open a tab itself.
    expect(anchor?.getAttribute('target')).toBe('_blank');
    expect(anchor?.getAttribute('rel')).toBe('noreferrer');
    // In a browser the default must run, so the anchor's own target="_blank"
    // opens a tab. Intercepting here would replace working behaviour.
    let defaultPrevented: boolean | null = null;
    document.addEventListener(
      'click',
      (event) => {
        defaultPrevented = event.defaultPrevented;
      },
      { once: true },
    );
    await userEvent.click(anchor as Element);
    expect(isDesktopShell()).toBe(false);
    expect(defaultPrevented, 'a browser-tab click must keep its default').toBe(false);
  });

  it('does not hand an in-app note link to the browser', async () => {
    const shell = withDesktopShell();
    const opened: string[] = [];
    render(
      <PreviewPane
        body="[note](notrios://databases/db_a/documents/doc_b)"
        themeBase="light"
        onOpenDocument={noop}
        onOpenStableLink={(uri) => opened.push(uri)}
        onError={noop}
      />,
    );
    const anchor = document.querySelector('a[data-app-uri^="notrios://"]');
    expect(anchor, 'the preview did not render a stable-link anchor').not.toBeNull();
    await userEvent.click(anchor as Element);
    // Routed in-app, never to the system browser.
    expect(shell.calls).toEqual([]);
    expect(opened).toEqual(['notrios://databases/db_a/documents/doc_b']);
    shell.restore();
  });
});
