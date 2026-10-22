import { render, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PreviewPane } from '../components/PreviewPane';
import { normalizePreviewHTML } from '../preview-utils';

// v1.0 J48. The preview sanitises twice -- md-editor-rt's sanitize prop, then
// its own renderer -- and the second pass used to remove the content URL the
// first made of resource://, so no attachment showed. The rule now leaves its
// own output as it found it, and only that.

const PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==';
const R = 'resource://default/resources';

// J34's media corpus, every link form the preview routes, and the forms it
// refuses. Each is sanitised once and twice.
const corpus = [
  `<img src="${R}/res_a" alt="a">`,
  `<img src="https://remote.example/img.png" srcset="https://remote.example/2x.png 2x">`,
  `<img src="${PNG}">`,
  `<video src="${R}/res_v" poster="${R}/res_p"><source src="${R}/res_s"><track src="${R}/res_t"></video>`,
  `<video src="https://remote.example/v.mp4" poster="https://remote.example/p.png"></video>`,
  `<audio src="${R}/res_au"></audio>`,
  `<svg><image href="${R}/res_svg"/><image xlink:href="https://remote.example/x.png"/><use href="https://remote.example/s.svg#i"/></svg>`,
  `<embed src="${R}/res_e"><object data="${R}/res_o"></object>`,
  `<a href="document://default/documents/doc_x">note</a>`,
  `<a href="${R}/res_a">attachment</a>`,
  `<a href="notrios://database/db_x/documents/doc_x">stable</a>`,
  `<a href="https://example.org/">web</a>`,
  `<a href="mailto:a@example.org">mail</a>`,
  `<a href="#section">section</a>`,
  `<a href="relative/path.md">relative</a>`,
  `<a href="javascript:alert(1)">script</a>`,
  `<img src="${R}/res_a" onerror="alert(1)" style="x">`,
  '<pre><code class="language-note-query">query: tag:x</code></pre>',
];

describe('J48-A: sanitising twice is sanitising once', () => {
  for (const html of corpus) {
    it(html, () => {
      const once = normalizePreviewHTML(html);
      expect(normalizePreviewHTML(once)).toBe(once);
    });
  }

  it('holds for the whole corpus as one page', () => {
    const once = normalizePreviewHTML(corpus.join('\n'));
    expect(normalizePreviewHTML(once)).toBe(once);
  });

  it('keeps every attachment it resolved, on every attribute', () => {
    const host = document.createElement('div');
    host.innerHTML = normalizePreviewHTML(normalizePreviewHTML(corpus.join('\n')));
    expect(host.querySelector('img[alt="a"]')?.getAttribute('src')).toBe('/api/v1/resources/res_a/content');
    const video = host.querySelector('video')!;
    expect(video.getAttribute('src')).toBe('/api/v1/resources/res_v/content');
    expect(video.getAttribute('poster')).toBe('/api/v1/resources/res_p/content');
    expect(host.querySelector('track')?.getAttribute('src')).toBe('/api/v1/resources/res_t/content');
    expect(host.querySelector('audio')?.getAttribute('src')).toBe('/api/v1/resources/res_au/content');
    expect(host.querySelector('image')?.getAttribute('href')).toBe('/api/v1/resources/res_svg/content');
    const link = Array.from(host.querySelectorAll('a')).find((a) => a.textContent === 'attachment')!;
    expect(link.getAttribute('href')).toBe('/api/v1/resources/res_a/content?download=1');
  });
});

describe('J48-A: only its own output is kept', () => {
  const forged: Array<[string, string]> = [
    ['another attachment', `<img data-app-src="${R}/res_a" src="/api/v1/resources/res_b/content">`],
    ['another path', `<img data-app-src="${R}/res_a" src="/etc/passwd">`],
    ['another origin', `<img data-app-src="${R}/res_a" src="//evil.example/api/v1/resources/res_a/content">`],
    ['the download form on an image', `<img data-app-src="${R}/res_a" src="/api/v1/resources/res_a/content?download=1">`],
    ['a remote URL', `<img data-app-src="${R}/res_a" src="https://evil.example/x.png">`],
    ['a record on another attribute', `<video data-app-poster="${R}/res_a" src="/api/v1/resources/res_a/content"></video>`],
  ];
  for (const [name, html] of forged) {
    it(`refuses a forged pair naming ${name}`, () => {
      const host = document.createElement('div');
      host.innerHTML = normalizePreviewHTML(html);
      const element = host.querySelector('img, video')!;
      expect(element.hasAttribute('src')).toBe(false);
    });
  }

  it('refuses a script link whatever app URI it claims', () => {
    const host = document.createElement('div');
    host.innerHTML = normalizePreviewHTML('<a data-app-uri="document://default/documents/doc_x" href="javascript:alert(1)">x</a>');
    expect(host.querySelector('a')!.hasAttribute('href')).toBe(false);
  });

  it('keeps a pair exactly as it would write it, which loads nothing a resource:// embed could not', () => {
    const host = document.createElement('div');
    host.innerHTML = normalizePreviewHTML(`<img data-app-src="${R}/res_a" src="/api/v1/resources/res_a/content">`);
    expect(host.querySelector('img')!.getAttribute('src')).toBe('/api/v1/resources/res_a/content');
  });
});

describe('J48: attachments show in the preview pane', () => {
  it('renders a Markdown and a wiki attachment embed with their content URLs, and a link with its download URL', async () => {
    const view = render(
      <PreviewPane
        body={`![md](${R}/res_a)\n\n![[${R}/res_a|wiki]]\n\n[download](${R}/res_a)\n`}
        themeBase="light" onOpenDocument={() => {}} onOpenStableLink={() => {}} onError={() => {}}
      />,
    );
    await waitFor(() => expect(view.container.querySelectorAll('#notrios-preview img')).toHaveLength(2));
    for (const image of Array.from(view.container.querySelectorAll('#notrios-preview img'))) {
      expect(image.getAttribute('src')).toBe('/api/v1/resources/res_a/content');
    }
    const link = Array.from(view.container.querySelectorAll('#notrios-preview a')).find((a) => a.textContent === 'download')!;
    expect(link.getAttribute('href')).toBe('/api/v1/resources/res_a/content?download=1');
  });
});
