// Shared preview/HTML helpers: sanitization, app-URI parsing, and snippet
// rendering. Extracted from App.tsx so panes can share them.
import { resourceContentURL } from './api';
import { isStableLink } from './stable-links';

export function normalizePreviewHTML(html: string): string {
  if (typeof DOMParser === 'undefined') return html;
  const parser = new DOMParser();
  const doc = parser.parseFromString(html, 'text/html');

  doc.querySelectorAll('script, style, iframe, object, embed, form, input, button, meta, link').forEach((node) => node.remove());

  doc.body.querySelectorAll('*').forEach((element) => {
    for (const attr of Array.from(element.attributes)) {
      const name = attr.name.toLowerCase();
      if (name.startsWith('on')) element.removeAttribute(attr.name);
      if (name === 'style') element.removeAttribute(attr.name);
    }
  });

  doc.querySelectorAll<HTMLAnchorElement>('a[href]').forEach((anchor) => {
    const href = anchor.getAttribute('href') || '';
    if (href.startsWith('document://')) {
      anchor.setAttribute('data-app-uri', href);
      anchor.setAttribute('href', '#');
      return;
    }
    if (href.startsWith('resource://')) {
      anchor.setAttribute('data-app-uri', href);
      const resourceID = parseResourceIDFromURI(href);
      if (resourceID) anchor.setAttribute('href', resourceContentURL(resourceID, true));
      return;
    }
    // An external notrios:// link is routed, never followed: it may name
    // another database, and only the server can say which note it means.
    if (isStableLink(href)) {
      anchor.setAttribute('data-app-uri', href);
      anchor.setAttribute('href', '#');
      return;
    }
    if (href.startsWith('http://') || href.startsWith('https://') || href.startsWith('mailto:') || href.startsWith('#')) {
      anchor.setAttribute('rel', 'noreferrer');
      if (href.startsWith('http')) anchor.setAttribute('target', '_blank');
      return;
    }
    anchor.removeAttribute('href');
  });

  doc.querySelectorAll<HTMLImageElement>('img[src]').forEach((image) => {
    const src = image.getAttribute('src') || '';
    if (src.startsWith('resource://')) {
      image.setAttribute('data-app-uri', src);
      const resourceID = parseResourceIDFromURI(src);
      if (resourceID) image.setAttribute('src', resourceContentURL(resourceID, false));
      return;
    }
    if (src.startsWith('http://') || src.startsWith('https://')) return;
    if (src.startsWith('data:image/')) return;
    image.removeAttribute('src');
  });

  return doc.body.innerHTML;
}

export function parseDocumentIDFromURI(uri: string): string | null {
  try {
    const parsed = new URL(uri);
    const parts = parsed.pathname.split('/').filter(Boolean);
    const index = parts.indexOf('documents');
    if (index >= 0 && parts[index + 1]) return decodeURIComponent(parts[index + 1]);
  } catch {
    // Fall through to regex fallback.
  }
  const match = uri.match(/documents\/([^/#?]+)/);
  return match ? decodeURIComponent(match[1]) : null;
}

export function parseResourceIDFromURI(uri: string): string | null {
  try {
    const parsed = new URL(uri);
    const parts = parsed.pathname.split('/').filter(Boolean);
    const index = parts.indexOf('resources');
    if (index >= 0 && parts[index + 1]) return decodeURIComponent(parts[index + 1]);
  } catch {
    // Fall through to regex fallback.
  }
  const match = uri.match(/resources\/([^/#?]+)/);
  return match ? decodeURIComponent(match[1]) : null;
}

export function Snippet({ htmlSnippet }: { htmlSnippet: string }) {
  const parts = htmlSnippet.split(/(<\/?mark>)/g);
  let marked = false;
  return (
    <span>
      {parts.map((part, index) => {
        if (part === '<mark>') {
          marked = true;
          return null;
        }
        if (part === '</mark>') {
          marked = false;
          return null;
        }
        return marked ? <mark key={index}>{part}</mark> : <span key={index}>{part}</span>;
      })}
    </span>
  );
}

export function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}
