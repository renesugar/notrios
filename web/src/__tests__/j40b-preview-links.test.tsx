import { fireEvent, render, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { CheckedLink } from '../api';
import { PreviewPane, type PreviewLinkCheck } from '../components/PreviewPane';
import { AMBIGUOUS_CLASS, UNRESOLVED_CLASS, applyLinkMarks, type LinkClassification } from '../preview-links';
import { describeLinkStatus } from '../useLinkIntelligence';

// v1.0 J40-B, to the rendering UI_DESIGN.md sets out under "Wiki links, and
// links that will not open". The classification is a buffer check as the
// service returns it: byte offsets into the body it checked.

const encoder = new TextEncoder();

/** A checked link for the first occurrence of `source` in `body`. */
function checked(body: string, source: string, status: string, extra: Partial<CheckedLink> = {}): CheckedLink {
  const index = body.indexOf(source);
  if (index < 0) throw new Error(`${source} is not in the body`);
  const start = encoder.encode(body.slice(0, index)).length;
  return {
    raw_target: source,
    status,
    start_byte: start,
    end_byte: start + encoder.encode(source).length,
    line: 1,
    column: 1,
    ...extra,
  };
}

function preview(body: string, linkCheck?: PreviewLinkCheck | null, onOpenDocument = vi.fn()) {
  const view = render(
    <PreviewPane
      body={body}
      themeBase="light"
      onOpenDocument={onOpenDocument}
      onOpenStableLink={() => {}}
      onError={() => {}}
      linkCheck={linkCheck}
    />,
  );
  const root = () => view.container.querySelector('#notrios-preview') as HTMLElement;
  return { ...view, root, onOpenDocument };
}

async function rendered(root: () => HTMLElement | null, text: string) {
  await waitFor(() => expect(root()?.textContent ?? '').toContain(text));
}

describe('J40-B: wiki links render', () => {
  it('renders a URI wiki link as the routed link its Markdown form is, showing the text after the pipe', async () => {
    const body = 'See [[document://default/documents/doc_x|shown]] and [[document://default/documents/doc_y]].\n';
    const { root, onOpenDocument } = preview(body);
    await rendered(root, 'See shown and');
    const shown = Array.from(root().querySelectorAll('a')).find((anchor) => anchor.textContent === 'shown')!;
    expect(shown.getAttribute('data-app-uri')).toBe('document://default/documents/doc_x');
    expect(root().textContent).toContain('document://default/documents/doc_y');
    expect(root().textContent).not.toContain('[[');
    fireEvent.click(shown);
    expect(onOpenDocument).toHaveBeenCalledWith('doc_x');
  });

  it('leaves a note embed as the text it was; an attachment embed is J47\'s', async () => {
    const body = 'Note ![[document://default/documents/doc_z]] here.\n';
    const { root } = preview(body);
    await rendered(root, 'Note ![[document://default/documents/doc_z]] here.');
    expect(root().querySelector('img')).toBeNull();
    expect(root().querySelector('a')).toBeNull();
  });

  it('keeps a Markdown link whose text holds a wiki link one Markdown link (J37)', async () => {
    const { root } = preview('A [see [[x]]](document://default/documents/doc_y) b.\n');
    await rendered(root, 'see');
    const anchors = root().querySelectorAll('a');
    expect(anchors).toHaveLength(1);
    expect(anchors[0].textContent).toBe('see [[x]]');
    expect(anchors[0].getAttribute('data-app-uri')).toBe('document://default/documents/doc_y');
  });

  it('turns no note text into markup', async () => {
    const body = 'X [[a"><img src=x onerror=alert(1)>|b<script>c</script>]] y.\n';
    const { root } = preview(body, { body, links: [checked(body, '[[a"><img src=x onerror=alert(1)>|b<script>c</script>]]', 'ambiguous')] });
    await rendered(root, 'X ');
    expect(root().querySelector('img')).toBeNull();
    expect(root().querySelector('script')).toBeNull();
    expect(root().querySelector('[onerror]')).toBeNull();
    expect(root().textContent).toContain('b<script>c</script>');
    await waitFor(() => expect(root().querySelector(`.${AMBIGUOUS_CLASS}`)).not.toBeNull());
  });
});

describe('J40-B: links that will not open are marked', () => {
  const body = [
    'Wiki [[index]] and [[index|shown]].',
    '',
    'Markdown [m](index.md) and embed ![[pic.png]].',
    '',
    'Missing [[document://default/documents/doc_gone|gone]].',
    '',
    'Code `[[index]]` and the word index.',
    '',
  ].join('\n');
  const check: PreviewLinkCheck = {
    body,
    links: [
      checked(body, '[[index]]', 'ambiguous'),
      checked(body, '[[index|shown]]', 'ambiguous'),
      checked(body, '[m](index.md)', 'unresolved'),
      checked(body, '![[pic.png]]', 'ambiguous'),
      checked(body, '[[document://default/documents/doc_gone|gone]]', 'unresolved'),
    ],
  };

  it('marks each by its status, with the editor\'s wording and no click target', async () => {
    const { root } = preview(body, check);
    await rendered(root, 'Wiki index and shown.');
    await waitFor(() => expect(root().querySelectorAll(`.${AMBIGUOUS_CLASS}`)).toHaveLength(3));
    const byText = (text: string) => Array.from(root().querySelectorAll<HTMLElement>('[data-link-source]')).find((element) => element.textContent === text)!;

    for (const text of ['index', 'shown']) {
      expect(byText(text).classList.contains(AMBIGUOUS_CLASS)).toBe(true);
      expect(byText(text).getAttribute('title')).toBe(describeLinkStatus('ambiguous'));
    }
    expect(byText('m').classList.contains(UNRESOLVED_CLASS)).toBe(true);
    expect(byText('m').getAttribute('title')).toBe(describeLinkStatus('unresolved'));
    expect(byText('![[pic.png]]').classList.contains(AMBIGUOUS_CLASS)).toBe(true);

    // A written-out URI the check says opens nothing is not a click target.
    const gone = byText('gone');
    expect(gone.classList.contains(UNRESOLVED_CLASS)).toBe(true);
    expect(gone.hasAttribute('href')).toBe(false);
    expect(gone.hasAttribute('data-app-uri')).toBe(false);
  });

  it('marks nothing it did not render as a link: not code, not a word', async () => {
    const { root } = preview(body, check);
    await rendered(root, 'Code');
    await waitFor(() => expect(root().querySelectorAll(`.${AMBIGUOUS_CLASS}`)).toHaveLength(3));
    const code = root().querySelector('code')!;
    expect(code.textContent).toBe('[[index]]');
    expect(code.querySelector('[data-link-source]')).toBeNull();
    expect(root().querySelectorAll(`.${AMBIGUOUS_CLASS}, .${UNRESOLVED_CLASS}`)).toHaveLength(5);
  });

  it('marks nothing without a check, as for a read-only note', async () => {
    const { root } = preview(body, null);
    await rendered(root, 'Wiki index');
    expect(root().querySelectorAll(`.${AMBIGUOUS_CLASS}, .${UNRESOLVED_CLASS}`)).toHaveLength(0);
    expect(root().querySelector('[title]')).toBeNull();
  });

  it('gives back what a mark took when a newer check disagrees', async () => {
    const view = preview(body, check);
    await waitFor(() => expect(view.root().querySelectorAll(`.${UNRESOLVED_CLASS}`)).toHaveLength(2));
    const newer: PreviewLinkCheck = {
      body,
      links: [checked(body, '[[document://default/documents/doc_gone|gone]]', 'resolved', { target_document_id: 'doc_gone' })],
    };
    view.rerender(
      <PreviewPane body={body} themeBase="light" onOpenDocument={view.onOpenDocument} onOpenStableLink={() => {}} onError={() => {}} linkCheck={newer} />,
    );
    await waitFor(() => expect(view.root().querySelectorAll(`.${AMBIGUOUS_CLASS}, .${UNRESOLVED_CLASS}`)).toHaveLength(0));
    const gone = Array.from(view.root().querySelectorAll('a')).find((anchor) => anchor.textContent === 'gone')!;
    expect(gone.getAttribute('data-app-uri')).toBe('document://default/documents/doc_gone');
    expect(gone.hasAttribute('title')).toBe(false);
    expect(gone.hasAttribute('class')).toBe(false);
  });

  it('keeps a title the author wrote, and gives it back after a mark', async () => {
    const titled = 'A [t](index.md "Mine") b.\n';
    const view = preview(titled, { body: titled, links: [checked(titled, '[t](index.md "Mine")', 'unresolved')] });
    const anchor = () => view.root().querySelector('a')!;
    await waitFor(() => expect(anchor().getAttribute('title')).toBe(describeLinkStatus('unresolved')));
    view.rerender(
      <PreviewPane body={titled} themeBase="light" onOpenDocument={() => {}} onOpenStableLink={() => {}} onError={() => {}} linkCheck={null} />,
    );
    await waitFor(() => expect(anchor().getAttribute('title')).toBe('Mine'));
  });
});

describe('J40-B: a name link the check resolves becomes a link', () => {
  it('routes it to what the check found', async () => {
    const body = 'Go to [[Alpha]].\n';
    const { root, onOpenDocument } = preview(body, {
      body,
      links: [checked(body, '[[Alpha]]', 'resolved', { target_document_id: 'doc_alpha', target_uri: 'document://default/documents/doc_alpha' })],
    });
    await waitFor(() => expect(root().querySelector('a[data-app-uri]')).not.toBeNull());
    const anchor = root().querySelector('a[data-app-uri]')!;
    expect(anchor.textContent).toBe('Alpha');
    fireEvent.click(anchor);
    expect(onOpenDocument).toHaveBeenCalledWith('doc_alpha');
  });

  it('places a link after multi-byte text by its byte offsets', async () => {
    const body = 'Ünïcödé — 日本 [[index]] end.\n';
    const { root } = preview(body, { body, links: [checked(body, '[[index]]', 'ambiguous')] });
    await waitFor(() => expect(root().querySelector(`.${AMBIGUOUS_CLASS}`)?.textContent).toBe('index'));
  });
});

describe('J40-B: a mark is undone in place', () => {
  // In the app a newer check usually arrives with a re-rendered preview, so the
  // undo is seldom reached through the pane. It is pinned here on the function,
  // where nothing re-renders behind it.
  it('gives back the destination and title a mark took, and takes off only its own', () => {
    const container = document.createElement('div');
    container.innerHTML =
      '<a data-link-source="[t](document://x)" href="#" data-app-uri="document://default/documents/doc_t" title="Mine" class="kept">t</a>' +
      '<a data-link-source="[[n]]">n</a>';
    const link = (status: string, extra: Partial<CheckedLink> = {}): CheckedLink => ({ raw_target: 'x', status, start_byte: 0, end_byte: 0, line: 1, column: 1, ...extra });
    const broken: LinkClassification = new Map([['[t](document://x)', link('unresolved')], ['[[n]]', link('ambiguous')]]);
    applyLinkMarks(container, broken);
    const [t, n] = Array.from(container.querySelectorAll('a'));
    expect(t.hasAttribute('href')).toBe(false);
    expect(t.hasAttribute('data-app-uri')).toBe(false);
    expect(t.getAttribute('title')).toBe(describeLinkStatus('unresolved'));
    expect(n.classList.contains(AMBIGUOUS_CLASS)).toBe(true);

    applyLinkMarks(container, new Map([['[[n]]', link('resolved', { target_uri: 'document://default/documents/doc_n' })]]));
    expect(t.getAttribute('href')).toBe('#');
    expect(t.getAttribute('data-app-uri')).toBe('document://default/documents/doc_t');
    expect(t.getAttribute('title')).toBe('Mine');
    expect(t.className).toBe('kept');
    expect(n.hasAttribute('class')).toBe(false);
    expect(n.getAttribute('data-app-uri')).toBe('document://default/documents/doc_n');

    applyLinkMarks(container, null);
    expect(n.hasAttribute('data-app-uri')).toBe(false);
    expect(n.hasAttribute('href')).toBe(false);
    const attributes = (element: Element) => Array.from(element.attributes).map((attribute) => `${attribute.name}=${attribute.value}`).sort();
    expect(attributes(t)).toEqual(['class=kept', 'data-app-uri=document://default/documents/doc_t', 'data-link-source=[t](document://x)', 'href=#', 'title=Mine']);
    expect(attributes(n)).toEqual(['data-link-source=[[n]]']);
  });
});
