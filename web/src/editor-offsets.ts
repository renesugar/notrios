// Byte offsets from the service, character indices in the editor.
//
// `POST /api/v1/links/check` locates every link by **UTF-8 byte** offset,
// because that is what Go measures and what the canonical link records store.
// CodeMirror positions are **UTF-16 code unit** indices, because that is what a
// JavaScript string is. The two agree exactly as long as a note is ASCII and
// disagree the moment it is not: `é` is two bytes and one unit, an emoji is
// four bytes and two units.
//
// Marking a link with an unconverted byte offset therefore underlines the wrong
// text in any note containing an accent — and the further into the note the
// link is, the further off the mark. This module is the conversion.

/** UTF-8 length of one code point. */
function utf8Length(codePoint: number): number {
  if (codePoint < 0x80) return 1;
  if (codePoint < 0x800) return 2;
  if (codePoint < 0x10000) return 3;
  return 4;
}

/**
 * Maps UTF-8 byte offsets in `text` to UTF-16 indices, in one pass.
 *
 * Offsets are resolved together rather than one at a time so a note with a
 * thousand links costs one walk of the text rather than a thousand.
 *
 * An offset that falls *inside* a multi-byte character is not in the result:
 * there is no character index that means "half of an é". Callers treat a
 * missing offset as "do not mark this one", which is the safe reading — a
 * missing underline is a smaller error than one drawn in the wrong place.
 */
export function byteOffsetsToIndices(text: string, byteOffsets: readonly number[]): Map<number, number> {
  const wanted = [...new Set(byteOffsets)].filter((offset) => offset >= 0).sort((a, b) => a - b);
  const indices = new Map<number, number>();
  let byte = 0;
  let index = 0;
  let next = 0;
  for (;;) {
    while (next < wanted.length && wanted[next] <= byte) {
      if (wanted[next] === byte) indices.set(wanted[next], index);
      next += 1;
    }
    if (next >= wanted.length || index >= text.length) break;
    const codePoint = text.codePointAt(index) as number;
    byte += utf8Length(codePoint);
    index += codePoint > 0xffff ? 2 : 1;
  }
  return indices;
}

/** One link's span in editor coordinates. */
export interface EditorSpan {
  from: number;
  to: number;
}

/**
 * Converts located spans into editor coordinates, dropping any that cannot be
 * placed exactly. A span is kept only when both ends land on a character
 * boundary and the range is non-empty and inside the text.
 */
export function spansToEditorRanges(
  text: string,
  spans: ReadonlyArray<{ start_byte: number; end_byte: number }>,
): EditorSpan[] {
  const offsets: number[] = [];
  for (const span of spans) {
    offsets.push(span.start_byte, span.end_byte);
  }
  const indices = byteOffsetsToIndices(text, offsets);
  const ranges: EditorSpan[] = [];
  for (const span of spans) {
    const from = indices.get(span.start_byte);
    const to = indices.get(span.end_byte);
    if (from === undefined || to === undefined) continue;
    if (to <= from || to > text.length) continue;
    ranges.push({ from, to });
  }
  return ranges;
}

/** Finds the span containing an editor position, or null. */
export function spanAt<T extends EditorSpan>(spans: readonly T[], position: number): T | null {
  for (const span of spans) {
    if (position >= span.from && position <= span.to) return span;
  }
  return null;
}
