// Preview pane: the rendered Markdown for the editor's body state. Uses the
// standalone MdPreview component from the locked md-editor-rt version so the
// editor (source of truth) and preview are independently sized panes.
// Sanitization and document://+resource:// link interception carry over via
// the shared preview renderer.
import { useMemo, type MouseEvent } from 'react';
import { MdPreview, type PreviewRendererProps } from 'md-editor-rt';
import type { ThemeMode } from '../themes';
import { normalizePreviewHTML, parseDocumentIDFromURI, parseResourceIDFromURI } from '../preview-utils';
import { resourceContentURL } from '../api';
import { isStableLink, parseStableLink } from '../stable-links';

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
      }
    }
    return function PreviewRenderer({ html, id, className }: PreviewRendererProps) {
      return <div id={id} className={className} onClick={onPreviewClick} dangerouslySetInnerHTML={{ __html: normalizePreviewHTML(html) }} />;
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
          noMermaid
        />
      </div>
    </section>
  );
}
