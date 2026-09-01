// Preview pane: the rendered Markdown for the editor's body state. Uses the
// standalone MdPreview component from the locked md-editor-rt version so the
// editor (source of truth) and preview are independently sized panes.
// Sanitization and document://+resource:// link interception carry over via
// the shared preview renderer.
import { useEffect, useMemo, useRef, type MouseEvent } from 'react';
import { MdPreview, type PreviewRendererProps } from 'md-editor-rt';
import type { ThemeMode } from '../themes';
import { normalizePreviewHTML, parseDocumentIDFromURI, parseResourceIDFromURI } from '../preview-utils';
import { resourceContentURL } from '../api';
import { isStableLink, parseStableLink } from '../stable-links';
import { disabledEditorExtensions, installEditorAssets } from '../editor-assets';
import { renderNoteQueryBlocks } from '../note-query';
import { openExternalURL } from '../desktop';

installEditorAssets();

export interface PreviewPaneProps {
  body: string;
  themeBase: ThemeMode;
  onOpenDocument: (documentID: string) => void;
  onOpenStableLink: (uri: string) => void;
  onError: (message: string) => void;
}

export function PreviewPane({ body, themeBase, onOpenDocument, onOpenStableLink, onError }: PreviewPaneProps) {
  // The renderer identity must be stable across keystrokes; recreate it only
  // when the navigation callbacks change.
  const Renderer = useMemo(() => {
    function onPreviewClick(event: MouseEvent<HTMLDivElement>) {
      const target = event.target;
      if (!(target instanceof Element)) return;
      const anchor = target.closest('a');
      if (!anchor) return;
      const appURI = anchor.getAttribute('data-app-uri') || anchor.getAttribute('href') || '';
      if (appURI.startsWith('document://')) {
        event.preventDefault();
        const documentID = parseDocumentIDFromURI(appURI);
        if (!documentID) {
          onError(`Could not parse document link: ${appURI}`);
          return;
        }
        onOpenDocument(documentID);
        return;
      }
      // A stable link may name another database, so it is handed to the
      // service to resolve rather than parsed into a local document ID here.
      if (isStableLink(appURI)) {
        event.preventDefault();
        if (!parseStableLink(appURI)) {
          onError(`Could not parse stable link: ${appURI}`);
          return;
        }
        onOpenStableLink(appURI);
        return;
      }
      if (appURI.startsWith('resource://')) {
        event.preventDefault();
        const resourceID = parseResourceIDFromURI(appURI);
        if (!resourceID) {
          onError(`Could not parse resource link: ${appURI}`);
          return;
        }
        window.location.assign(resourceContentURL(resourceID, true));
        return;
      }
      // A remote link leaves the application. In a browser the anchor's own
      // `target="_blank"` already opens a tab, so this does nothing and the
      // default runs. In the Wails webview nothing handles a `_blank` click,
      // so the URL is handed to the system browser instead of being silently
      // swallowed — the alternative is the user copying it out by hand.
      //
      // Suppressing the default matters as much as opening the browser: a
      // webview that followed the link in place would replace the running
      // application with a website.
      if (openExternalURL(appURI)) {
        event.preventDefault();
      }
    }
    return function PreviewRenderer({ html, id, className }: PreviewRendererProps) {
      const containerRef = useRef<HTMLDivElement>(null);
      const sanitized = useMemo(() => normalizePreviewHTML(html), [html]);
      // Query blocks are filled in *after* the note has rendered, and the
      // cleanup aborts anything still in flight — a late reply must not write
      // into a preview that has since moved to another note.
      useEffect(() => {
        if (!containerRef.current) return;
        return renderNoteQueryBlocks(containerRef.current);
      }, [sanitized]);
      return <div ref={containerRef} id={id} className={className} onClick={onPreviewClick} dangerouslySetInnerHTML={{ __html: sanitized }} />;
    };
  }, [onOpenDocument, onOpenStableLink, onError]);

  return (
    <section className="pane preview-pane" aria-label="Markdown preview" data-testid="pane-preview">
      <div className="pane-scroll">
        <MdPreview
          id="notrios-preview"
          value={body}
          theme={themeBase}
          previewTheme="github"
          language="en-US"
          sanitize={normalizePreviewHTML}
          previewComponent={Renderer}
          {...disabledEditorExtensions}
        />
      </div>
    </section>
  );
}
