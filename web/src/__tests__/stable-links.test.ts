import { describe, expect, it } from 'vitest';
import {
  formatStableLink,
  isStableLink,
  parseDeepLinkHash,
  parseStableLink,
  stableLinkStatusMessage,
} from '../stable-links';
import { normalizePreviewHTML } from '../preview-utils';

describe('parseStableLink', () => {
  it('accepts the documented shape and round-trips format()', () => {
    const uri = formatStableLink('db_abc', 'doc_xyz');
    expect(uri).toBe('notrios://databases/db_abc/documents/doc_xyz');
    expect(parseStableLink(uri)).toEqual({ databaseID: 'db_abc', documentID: 'doc_xyz', anchor: '' });
  });

  it('keeps an anchor', () => {
    expect(parseStableLink('notrios://databases/db_a/documents/doc_b#heading-two')).toEqual({
      databaseID: 'db_a',
      documentID: 'doc_b',
      anchor: 'heading-two',
    });
  });

  it('rejects anything that is not exactly the documented shape', () => {
    const rejected = [
      'notrios://databases/db_a/documents/doc_b/extra',
      'notrios://databases/db_a/documents/',
      'notrios://databases//documents/doc_b',
      'notrios://profiles/db_a/documents/doc_b',
      'notrios://databases/db_a/resources/res_b',
      'notrios://databases/db_a/documents/doc_b?x=1',
      'notrios://databases/db_a/documents/../../etc/passwd',
      'notrios://databases/db a/documents/doc_b',
      `notrios://databases/db_a/documents/${'d'.repeat(200)}`,
      'document://default/documents/doc_b',
      'javascript:alert(1)',
      '',
    ];
    for (const uri of rejected) {
      expect(parseStableLink(uri), uri).toBeNull();
    }
  });

  it('reports the scheme without committing to validity', () => {
    expect(isStableLink('NOTRIOS://databases/db_a/documents/doc_b')).toBe(true);
    expect(isStableLink('https://example.com')).toBe(false);
  });
});

describe('parseDeepLinkHash', () => {
  it('reads the document the desktop handler resolved', () => {
    expect(parseDeepLinkHash('#document=doc_abc&anchor=heading')).toEqual({
      documentID: 'doc_abc',
      anchor: 'heading',
    });
    expect(parseDeepLinkHash('#document=doc_abc')).toEqual({ documentID: 'doc_abc', anchor: '' });
  });

  it('ignores hashes that are not deep links or carry an unusable ID', () => {
    for (const hash of ['', '#', '#other=1', '#document=', '#document=../etc/passwd', '#document=a b']) {
      expect(parseDeepLinkHash(hash), hash).toBeNull();
    }
  });
});

describe('preview link handling', () => {
  it('routes notrios:// links instead of letting the browser follow them', () => {
    const html = normalizePreviewHTML(
      '<a href="notrios://databases/db_a/documents/doc_b">stable</a>',
    );
    expect(html).toContain('data-app-uri="notrios://databases/db_a/documents/doc_b"');
    expect(html).toContain('href="#"');
  });

  it('never lets untrusted note HTML fetch remote image bytes directly', () => {
    const html = normalizePreviewHTML('<picture><source srcset="http://127.0.0.1/source.png"><img src="http://127.0.0.1/private.png" srcset="http://127.0.0.1/candidate.png 2x" alt="private"></picture>');
    const host = document.createElement('div');
    host.innerHTML = html;
    const image = host.querySelector('img');
    expect(host.querySelector('source')).toBeNull();
    expect(image?.getAttribute('src')).toBeNull();
    expect(image?.getAttribute('srcset')).toBeNull();
    expect(image?.getAttribute('data-remote-src')).toBe('http://127.0.0.1/private.png');
    expect(image?.classList.contains('remote-media-placeholder')).toBe(true);
  });
});

describe('stableLinkStatusMessage', () => {
  it('explains a wrong-database link as belonging elsewhere, not as broken', () => {
    const message = stableLinkStatusMessage('foreign_database', 'notrios://databases/db_x/documents/doc_y');
    expect(message).toContain('another Notrios database');
    expect(message).toContain('notrios://databases/db_x/documents/doc_y');
  });

  it('distinguishes a stale target', () => {
    expect(stableLinkStatusMessage('stale_target', 'uri')).toContain('no longer exists');
  });
});
