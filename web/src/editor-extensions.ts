// In-editor link intelligence, built on the CodeMirror 6 editor that
// `md-editor-rt` already runs.
//
// E6 set out to decide whether to migrate from `md-editor-rt` to CodeMirror 6.
// The premise was wrong: `md-editor-rt` 6.5.3 *is* CodeMirror 6. It depends on
// `@codemirror/{view,state,autocomplete,commands,language,search}` 6.x and it
// exposes them — `completions` takes `CompletionSource`s straight into
// `@codemirror/autocomplete`, `config({ codeMirrorExtensions })` accepts
// arbitrary extensions, `getEditorView()` hands back the `EditorView` itself,
// and `domEventHandlers` is CodeMirror's own handler map.
//
// So the three things E5 reported as out of reach — caret position, inline
// decorations, in-editor Ctrl-click — are reachable through the editor's public
// API, at no bundle cost, because the library is already there. This file is
// that proof, and it is the evidence behind `PROJECT_DECISIONS.md` 20.
import { autocompletion, type Completion, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete';
import { StateEffect, StateField, type Extension } from '@codemirror/state';
import { Decoration, EditorView, type DecorationSet } from '@codemirror/view';
import { suggestLinkTargets, type CheckedLink } from './api';
import { isBrokenStatus } from './useLinkIntelligence';
import { spansToEditorRanges, type EditorSpan } from './editor-offsets';

/** A broken link placed in editor coordinates. */
export interface BrokenSpan extends EditorSpan {
  status: string;
}

/** Replaces the marked links. Dispatched whenever a check returns. */
export const setBrokenLinks = StateEffect.define<BrokenSpan[]>();

const brokenLinkMark = Decoration.mark({ class: 'cm-notrios-broken-link' });

/**
 * Holds the broken-link decorations.
 *
 * Decorations are mapped through document changes, so an underline follows its
 * text while someone keeps typing instead of sitting at a stale offset until
 * the next check returns.
 */
const brokenLinkField = StateField.define<DecorationSet>({
  create() {
    return Decoration.none;
  },
  update(decorations, transaction) {
    let next = decorations.map(transaction.changes);
    for (const effect of transaction.effects) {
      if (!effect.is(setBrokenLinks)) continue;
      const marks = effect.value
        .slice()
        .sort((a, b) => a.from - b.from)
        .map((span) => brokenLinkMark.range(span.from, span.to));
      next = Decoration.set(marks, true);
    }
    return next;
  },
  provide: (field) => EditorView.decorations.from(field),
});

/**
 * The wavy underline. It is a class rather than an inline style so a custom
 * theme can restyle it, and it uses `text-decoration` rather than a background
 * so it cannot be confused with a selection.
 */
const brokenLinkTheme = EditorView.baseTheme({
  '.cm-notrios-broken-link': {
    textDecoration: 'underline wavy',
    textDecorationColor: '#d93025',
    textDecorationThickness: '1px',
  },
});

/** The extensions to hand to `md-editor-rt`'s `codeMirrorExtensions` hook. */
export function brokenLinkExtensions(): Extension[] {
  return [brokenLinkField, brokenLinkTheme];
}

/**
 * Converts a check result into editor coordinates and dispatches it.
 *
 * `checkedBody` is the text the service actually classified. If the buffer has
 * moved on, the offsets describe a document that no longer exists, so nothing is
 * marked rather than something being marked in the wrong place — the same rule
 * the publication rewriter and `notriosctl fix` follow for a moved span.
 */
export function applyBrokenLinks(view: EditorView, checkedBody: string, links: readonly CheckedLink[]): boolean {
  if (view.state.doc.toString() !== checkedBody) return false;
  const broken = links.filter((link) => isBrokenStatus(link.status));
  const spans = spansToEditorRanges(checkedBody, broken);
  const withStatus: BrokenSpan[] = spans.map((span, index) => ({
    ...span,
    status: broken[index]?.status ?? 'unresolved',
  }));
  view.dispatch({ effects: setBrokenLinks.of(withStatus) });
  return true;
}

/** Clears the marks, for when a check fails or the note changes. */
export function clearBrokenLinks(view: EditorView): void {
  view.dispatch({ effects: setBrokenLinks.of([]) });
}

/**
 * `[[` opens the note picker inline, which is the spelling every Obsidian and
 * Logseq user already has in their fingers. The completion inserts a canonical
 * Markdown link rather than a wikilink: a wikilink resolves by title and breaks
 * when the note is renamed, which is the defect `notriosctl fix` exists to
 * repair afterwards.
 */
const WIKILINK_TRIGGER = /\[\[([^\]\n]*)$/;

export function linkCompletionSource(excludeDocumentID?: () => string | undefined) {
  return async function completeLink(context: CompletionContext): Promise<CompletionResult | null> {
    const before = context.matchBefore(WIKILINK_TRIGGER);
    if (!before) return null;
    const query = before.text.slice(2);
    if (query.trim().length < 2) {
      // Below the service's minimum. Returning a result with no options keeps
      // the popup open so it fills in as the user keeps typing.
      return { from: before.from, options: [], filter: false };
    }
    // CodeMirror signals a superseded query through `aborted` plus an abort
    // listener rather than an AbortSignal, so the in-flight fetch is cancelled
    // by bridging the two.
    const controller = new AbortController();
    context.addEventListener('abort', () => controller.abort());
    let options: Completion[] = [];
    try {
      const response = await suggestLinkTargets(query, {
        excludeDocumentID: excludeDocumentID?.(),
        signal: controller.signal,
      });
      if (context.aborted) return null;
      options = (response.suggestions ?? []).map((suggestion) => ({
        label: suggestion.title,
        detail: suggestion.match === 'word_prefix' ? 'word match' : undefined,
        // The whole `[[query` is replaced, so nothing of the trigger is left
        // behind in the note.
        apply: `[${suggestion.title}](${suggestion.uri})`,
        type: 'text',
      }));
    } catch {
      // Degrade to no completions. An assist must not interrupt typing.
      return null;
    }
    return { from: before.from, options, filter: false };
  };
}

/** Autocomplete wired to the link source, for the `codeMirrorExtensions` hook. */
export function linkAutocompleteExtension(excludeDocumentID?: () => string | undefined): Extension {
  return autocompletion({ override: [linkCompletionSource(excludeDocumentID)] });
}

/**
 * Ctrl-click (Cmd-click on macOS) on a link opens its target.
 *
 * `posAtCoords` is what makes this possible and is exactly the source-position
 * access E5 reported as unavailable. It is available; E5 was wrong.
 */
export function linkClickHandler(
  spans: () => ReadonlyArray<{ from: number; to: number; documentID?: string }>,
  open: (documentID: string) => void,
) {
  return (event: MouseEvent, view: EditorView): boolean => {
    if (!event.ctrlKey && !event.metaKey) return false;
    const position = view.posAtCoords({ x: event.clientX, y: event.clientY });
    if (position === null) return false;
    for (const span of spans()) {
      if (position >= span.from && position <= span.to && span.documentID) {
        event.preventDefault();
        open(span.documentID);
        return true;
      }
    }
    return false;
  };
}
