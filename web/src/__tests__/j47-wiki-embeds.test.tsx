import { render, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { CheckedLink } from '../api';
import { PreviewPane, type PreviewLinkCheck } from '../components/PreviewPane';
import { UNRESOLVED_CLASS } from '../preview-links';

// v1.0 J47: a working wiki embed of an attachment renders as the Markdown
// embed of the same target, through the same neutralisation. The rule emits
// markdown-it's own image token, and these tests hold it to that by rendering
// both forms side by side and comparing what came out.

function preview(body: string, linkCheck?: PreviewLinkCheck | null) {
  const view = render(
    <PreviewPane body={body} themeBase="light" onOpenDocument={() => {}} onOpenStableLink={() => {}} onError={() => {}} linkCheck={linkCheck} />,
  );
  return () => view.container.querySelector('#notrios-preview') as HTMLElement;
}

/** Every attribute but the source text the marks are keyed by. */
function attributes(element: Element): Record<string, string> {
  const out: Record<string, string> = {};
  for (const attribute of Array.from(element.attributes)) {
    if (attribute.name !== 'data-link-source') out[attribute.name] = attribute.value;
  }
  return out;
}

describe('J47: a wiki embed of an attachment is its Markdown embed', () => {
  const pairs: Array<[string, string]> = [
    ['![[resource://default/resources/res_a]]', '![](resource://default/resources/res_a)'],
    ['![[resource://default/resources/res_a|A heron]]', '![A heron](resource://default/resources/res_a)'],
    ['![[resource://default/resources/res_a|300]]', '![300](resource://default/resources/res_a)'],
    ['![[resource://not-a-resource]]', '![](resource://not-a-resource)'],
  ];

  for (const [wiki, markdown] of pairs) {
    it(`${wiki} renders as ${markdown}`, async () => {
      const root = preview(`Wiki ${wiki}\n\nMarkdown ${markdown}\n`);
      await waitFor(() => expect(root()?.querySelectorAll('img')).toHaveLength(2));
      const [fromWiki, fromMarkdown] = Array.from(root().querySelectorAll('img'));
      expect(attributes(fromWiki)).toEqual(attributes(fromMarkdown));
      expect(fromWiki.hasAttribute('srcset')).toBe(false);
      expect(root().textContent).not.toContain('![[');
    });
  }

  it('keeps the attachment and its alt text, as the Markdown embed does', async () => {
    const root = preview('![[resource://default/resources/res_a|A heron]]\n');
    await waitFor(() => expect(root()?.querySelector('img')).not.toBeNull());
    const image = root().querySelector('img')!;
    expect(image.getAttribute('data-app-uri')).toBe('resource://default/resources/res_a');
    expect(image.getAttribute('alt')).toBe('A heron');
    // Its local content URL: since J48 a second sanitising pass keeps it.
    expect(image.getAttribute('src')).toBe('/api/v1/resources/res_a/content');
  });

  it('turns no alt text into markup', async () => {
    const root = preview('![[resource://default/resources/res_a|<img src=x onerror=alert(1)> *b*]]\n');
    await waitFor(() => expect(root()?.querySelector('img')).not.toBeNull());
    expect(root().querySelectorAll('img')).toHaveLength(1);
    expect(root().querySelector('[onerror]')).toBeNull();
  });
});

describe('J47: every other embed stays the text it was', () => {
  for (const embed of [
    '![[document://default/documents/doc_x]]',
    '![[notrios://database/db_x/documents/doc_x]]',
    '![[https://example.org/remote.png]]',
    '![[pic.png]]',
  ]) {
    it(`${embed} loads nothing`, async () => {
      const root = preview(`Embed ${embed} here.\n`);
      await waitFor(() => expect(root()?.textContent).toContain(`Embed ${embed} here.`));
      expect(root().querySelector('img, video, audio, iframe, object, embed, source')).toBeNull();
      expect(root().querySelector('[data-remote-src]')).toBeNull();
    });
  }
});

describe('J47: a broken attachment embed is marked like a link', () => {
  it('marks the image the check calls unresolved', async () => {
    const body = 'Gone ![[resource://default/resources/res_gone]].\n';
    const start = new TextEncoder().encode('Gone ').length;
    const source = '![[resource://default/resources/res_gone]]';
    const link: CheckedLink = { raw_target: 'resource://default/resources/res_gone', status: 'unresolved', start_byte: start, end_byte: start + source.length, line: 1, column: 6 };
    const root = preview(body, { body, links: [link] });
    await waitFor(() => expect(root()?.querySelector(`img.${UNRESOLVED_CLASS}`)).not.toBeNull());
    expect(root().querySelector('img')!.getAttribute('title')).toBe('no note or resource with this target');
  });
});
