import { describe, expect, it } from 'vitest';
import { normalizePreviewHTML } from '../preview-utils';

// J34: a note's remote media never loads in the preview until it is localized,
// whichever element names it. Inline images and inline SVG keep rendering.

const PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==';

function render(html: string): HTMLElement {
  const host = document.createElement('div');
  host.innerHTML = normalizePreviewHTML(html);
  return host;
}

/** Every attribute value in the rendered preview that would make a request. */
function loadingURLs(host: HTMLElement): string[] {
  const urls: string[] = [];
  host.querySelectorAll('*').forEach((element) => {
    if (element.localName === 'a') return; // navigation, not a load
    for (const name of ['src', 'srcset', 'poster', 'data', 'href', 'xlink:href']) {
      const value = element.getAttribute(name);
      if (value) urls.push(`${element.localName}[${name}]=${value}`);
    }
  });
  return urls;
}

describe('preview media references', () => {
  it('leaves no remote URL on any media element or attribute', () => {
    const host = render([
      '<video src="https://remote.example/v.mp4" poster="https://remote.example/p.png"><source src="https://remote.example/s.mp4"><track src="https://remote.example/t.vtt"></video>',
      '<audio src="http://remote.example/a.mp3"></audio>',
      '<svg><image href="https://remote.example/i.png"/><image xlink:href="https://remote.example/x.png"/></svg>',
      '<svg><filter id="f"><feImage href="https://remote.example/fe.png"/></filter><use href="https://remote.example/sprite.svg#icon"/></svg>',
      '<embed src="https://remote.example/e.pdf"><object data="https://remote.example/o.pdf"></object>',
      '<img src="https://remote.example/img.png" srcset="https://remote.example/2x.png 2x">',
    ].join('\n'));
    expect(loadingURLs(host).filter((url) => url.includes('remote.example'))).toEqual([]);
  });

  it('keeps each remote source as inert metadata', () => {
    const host = render('<video src="https://remote.example/v.mp4" poster="https://remote.example/p.png"></video><audio src="https://remote.example/a.mp3"></audio><video><track src="https://remote.example/t.vtt"></video><svg><image href="https://remote.example/i.png"/></svg>');
    const [video] = host.querySelectorAll('video');
    expect(video.getAttribute('data-remote-src')).toBe('https://remote.example/v.mp4');
    expect(video.getAttribute('data-remote-poster')).toBe('https://remote.example/p.png');
    expect(host.querySelector('audio')?.getAttribute('data-remote-src')).toBe('https://remote.example/a.mp3');
    expect(host.querySelector('track')?.getAttribute('data-remote-src')).toBe('https://remote.example/t.vtt');
    expect(host.querySelector('image')?.getAttribute('data-remote-href')).toBe('https://remote.example/i.png');
  });

  it('keeps inline images and inline SVG rendering', () => {
    const host = render(`<img src="${PNG}"><video poster="${PNG}"></video><svg width="1" height="1"><rect width="1" height="1"/><image href="${PNG}"/></svg>`);
    expect(host.querySelector('img')?.getAttribute('src')).toBe(PNG);
    expect(host.querySelector('video')?.getAttribute('poster')).toBe(PNG);
    expect(host.querySelector('svg rect')).not.toBeNull();
    expect(host.querySelector('image')?.getAttribute('href')).toBe(PNG);
  });

  it('resolves resource:// media to its content URL', () => {
    const host = render('<video src="resource://default/resources/res_video" poster="resource://default/resources/res_poster"></video><svg><image href="resource://default/resources/res_svg"/></svg>');
    const video = host.querySelector('video');
    expect(video?.getAttribute('src')).toContain('/resources/res_video');
    expect(video?.getAttribute('src')).not.toContain('resource://');
    expect(video?.getAttribute('poster')).toContain('/resources/res_poster');
    expect(host.querySelector('image')?.getAttribute('href')).toContain('/resources/res_svg');
  });

  it('removes any other scheme rather than guessing', () => {
    const host = render('<video src="javascript:alert(1)" poster="file:///etc/passwd"></video><svg><image href="ftp://remote.example/i.png"/></svg>');
    expect(loadingURLs(host)).toEqual([]);
  });

  it('leaves SVG links to the link rule', () => {
    const host = render('<svg><a href="https://remote.example/page"><text>go</text></a></svg>');
    expect(host.querySelector('a')?.getAttribute('href')).toBe('https://remote.example/page');
  });
});
