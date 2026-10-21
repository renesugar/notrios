// Wiki links in the preview, and the marks on links that will not open (v1.0
// J40-B; UI_DESIGN.md, "Wiki links, and links that will not open").
//
// Two halves. A markdown-it rule recognises wiki syntax while the preview
// renders, and records each link's exact source text on the element it emits;
// markdown-it's own link rule is wrapped to record the same for a Markdown
// link. After the HTML is in the page, `applyLinkMarks` looks each element's
// source text up in the editor's last buffer check and marks, routes or leaves
// it. Nothing is matched against rendered text, and nothing here decides what a
// link resolves to: that stays the service's canonical extractor and resolver.
import type MarkdownIt from 'markdown-it';
import type StateInline from 'markdown-it/lib/rules_inline/state_inline.mjs';
import type Token from 'markdown-it/lib/token.mjs';
import type { CheckedLink } from './api';
import { byteOffsetsToIndices } from './editor-offsets';
import { describeLinkStatus, isBrokenStatus } from './useLinkIntelligence';

/** The attribute that carries a link's source text from the renderer to the marks. */
export const LINK_SOURCE_ATTR = 'data-link-source';
export const AMBIGUOUS_CLASS = 'preview-link-ambiguous';
export const UNRESOLVED_CLASS = 'preview-link-unresolved';
/** Set on a name link this file routed, so a later check can take it back. */
const ROUTED_ATTR = 'data-link-routed';
/** Where a marked link's destination waits, so the next pass can give it back. */
const STASH_PREFIX = 'data-link-stashed-';
/** Set on an element this file marked, so only its own title is taken back. */
const MARKED_ATTR = 'data-link-marked';

/** The schemes a wiki link may name and still be rendered as a link without a check. */
const ROUTED_SCHEMES = ['document://', 'resource://', 'notrios://'];
/** What a mark takes off a link: its destination, and a title the author wrote. */
const STASHED = ['href', 'data-app-uri', 'title'];

const OPEN = 0x5b; // [
const CLOSE = 0x5d; // ]
const BANG = 0x21; // !
const NEWLINE = 0x0a;

/**
 * The wiki-link rule. It matches what the service's extractor does,
 * `(!?)\[\[([^\]\n]+)\]\]`, and splits the inner text at the first pipe the same
 * way. Inside a Markdown link's text it does nothing, as the extractor drops a
 * wiki link that overlaps a Markdown one (J37).
 */
function wikiLinkRule(state: StateInline, silent: boolean): boolean {
  // markdown-it counts how deep in a link's text it is; its types do not say so.
  if ((state as StateInline & { linkLevel: number }).linkLevel > 0) return false;
  const start = state.pos;
  const src = state.src;
  const embed = src.charCodeAt(start) === BANG;
  const open = embed ? start + 1 : start;
  if (src.charCodeAt(open) !== OPEN || src.charCodeAt(open + 1) !== OPEN) return false;
  let end = open + 2;
  while (end < state.posMax && src.charCodeAt(end) !== CLOSE && src.charCodeAt(end) !== NEWLINE) end++;
  if (end === open + 2 || src.charCodeAt(end) !== CLOSE || src.charCodeAt(end + 1) !== CLOSE) return false;
  const after = end + 2;
  // Silent mode is markdown-it measuring a Markdown link's text. Declining
  // there leaves `[see [[x]]](y)` the Markdown link it was before this rule,
  // which is also what the extractor keeps when a wiki link overlaps a
  // Markdown one (J37); the text is then tokenized inside the link, where the
  // rule does nothing.
  if (silent) return false;
  const source = src.slice(start, after);
  const inner = src.slice(open + 2, end).trim();
  const pipe = inner.indexOf('|');
  const target = (pipe < 0 ? inner : inner.slice(0, pipe)).trim();
  const display = pipe < 0 ? target : inner.slice(pipe + 1).trim();

  if (embed && target.startsWith('resource://')) {
    // An attachment embed is the Markdown embed of the same target: the same
    // token markdown-it's own image rule emits, so the same renderer and the
    // same media neutralisation follow by construction (v1.0 J47). The text
    // after the pipe is the alt text, as the label is for `![shown](...)`.
    const href = state.md.normalizeLink(target);
    if (state.md.validateLink(href)) {
      const children: Token[] = [];
      state.md.inline.parse(pipe < 0 ? '' : display, state.md, state.env, children);
      const image = state.push('image', 'img', 0);
      image.attrs = [['src', href], ['alt', ''], [LINK_SOURCE_ATTR, source]];
      image.children = children;
      image.content = pipe < 0 ? '' : display;
      state.pos = after;
      return true;
    }
  }
  if (embed) {
    // Any other embed stays the text it always was, wrapped so a broken one
    // can be marked: a note embed is transclusion, not media, and a name the
    // import left as written names nothing certain.
    const tokenOpen = state.push('wikilink_embed_open', 'span', 1);
    tokenOpen.attrs = [[LINK_SOURCE_ATTR, source]];
    state.push('text', '', 0).content = source;
    state.push('wikilink_embed_close', 'span', -1);
  } else {
    const tokenOpen = state.push('link_open', 'a', 1);
    tokenOpen.attrs = [[LINK_SOURCE_ATTR, source]];
    if (ROUTED_SCHEMES.some((scheme) => target.startsWith(scheme))) {
      const href = state.md.normalizeLink(target);
      if (state.md.validateLink(href)) tokenOpen.attrs.unshift(['href', href]);
    }
    state.push('text', '', 0).content = display;
    state.push('link_close', 'a', -1);
  }
  state.pos = after;
  return true;
}

/** Wraps markdown-it's link rule so a Markdown link records its source text too. */
function recordLinkSource(original: (state: StateInline, silent: boolean) => boolean) {
  return function link(state: StateInline, silent: boolean): boolean {
    const start = state.pos;
    const first = state.tokens.length;
    if (!original(state, silent)) return false;
    if (silent) return true;
    // Text pending before the link is flushed as a token of its own first, so
    // the link's opening token is the first link_open after `first`.
    const token = state.tokens.slice(first).find((candidate) => candidate.type === 'link_open');
    token?.attrSet(LINK_SOURCE_ATTR, state.src.slice(start, state.pos));
    return true;
  };
}

/** The markdown-it plugin: the wiki rule, and the link rule's source text. */
export function previewLinksPlugin(md: MarkdownIt): void {
  const rules = (md.inline.ruler as unknown as { __rules__: { name: string; fn: (state: StateInline, silent: boolean) => boolean }[] }).__rules__;
  const link = rules.find((rule) => rule.name === 'link');
  if (link) md.inline.ruler.at('link', recordLinkSource(link.fn));
  md.inline.ruler.before('link', 'notrios_wikilink', wikiLinkRule);
}

/** A buffer check, keyed by each link's exact source text. */
export type LinkClassification = ReadonlyMap<string, CheckedLink>;

export function classifyBySource(checkedBody: string, links: readonly CheckedLink[]): LinkClassification {
  const offsets: number[] = [];
  for (const link of links) offsets.push(link.start_byte, link.end_byte);
  const indices = byteOffsetsToIndices(checkedBody, offsets);
  const bySource = new Map<string, CheckedLink>();
  for (const link of links) {
    const from = indices.get(link.start_byte);
    const to = indices.get(link.end_byte);
    if (from === undefined || to === undefined || to <= from) continue;
    const source = checkedBody.slice(from, to);
    // Two links with the same text have the same status, since a status
    // depends on nothing else, so the first is as good as any.
    if (!bySource.has(source)) bySource.set(source, link);
  }
  return bySource;
}

/**
 * Marks, routes or leaves every element the renderer recorded a source for.
 * Run after each render and after each check, so it first undoes what it did
 * last time: a link the newer check no longer calls broken loses its mark.
 */
export function applyLinkMarks(container: HTMLElement, classification: LinkClassification | null): void {
  container.querySelectorAll<HTMLElement>(`[${LINK_SOURCE_ATTR}]`).forEach((element) => {
    restore(element);
    const link = classification?.get(element.getAttribute(LINK_SOURCE_ATTR) ?? '');
    if (!link) return;
    if (isBrokenStatus(link.status)) {
      // A marked link is not a click target, even one whose URI was written
      // out: the check says it opens nothing. The destination is kept aside so
      // a later check that disagrees can give it back.
      for (const name of STASHED) {
        const value = element.getAttribute(name);
        if (value === null) continue;
        element.setAttribute(`${STASH_PREFIX}${name}`, value);
        element.removeAttribute(name);
      }
      element.classList.add(link.status === 'ambiguous' ? AMBIGUOUS_CLASS : UNRESOLVED_CLASS);
      element.setAttribute('title', describeLinkStatus(link.status));
      element.setAttribute(MARKED_ATTR, '');
      return;
    }
    // A name link the check resolved becomes the link it names. An element
    // that already has a destination keeps it.
    const uri = link.target_uri ?? '';
    if (element.tagName === 'A' && !element.hasAttribute('href') && ROUTED_SCHEMES.some((scheme) => uri.startsWith(scheme))) {
      element.setAttribute('data-app-uri', uri);
      element.setAttribute('href', '#');
      element.setAttribute(ROUTED_ATTR, '');
    }
  });
}

/** Undoes whatever the last pass did to an element. */
function restore(element: HTMLElement): void {
  if (element.hasAttribute(MARKED_ATTR)) {
    element.removeAttribute(MARKED_ATTR);
    element.classList.remove(AMBIGUOUS_CLASS, UNRESOLVED_CLASS);
    if (element.classList.length === 0) element.removeAttribute('class');
    element.removeAttribute('title');
  }
  if (element.hasAttribute(ROUTED_ATTR)) {
    element.removeAttribute(ROUTED_ATTR);
    element.removeAttribute('data-app-uri');
    element.removeAttribute('href');
  }
  for (const name of STASHED) {
    const stashed = element.getAttribute(`${STASH_PREFIX}${name}`);
    if (stashed === null) continue;
    element.setAttribute(name, stashed);
    element.removeAttribute(`${STASH_PREFIX}${name}`);
  }
}
