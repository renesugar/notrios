import { describe, expect, it } from 'vitest';

// The resolver is an ESM JavaScript CLI intentionally kept outside the web
// bundle. Vitest can import its pure functions; tsc has no declaration file
// for this repository-only tool.
// @ts-ignore docaudit_ts.mjs is a tested repository script, not a web module.
import {
  DocAuditTypeScriptError,
  parseArguments,
  parseTypeScriptAnchor,
  resolveTypeScriptAnchor,
  resolveTypeScriptAnchors,
  resolveTypeScriptSourceText,
// @ts-ignore docaudit_ts.mjs is a tested repository script, not a web module.
} from '../../../scripts/docaudit_ts.mjs';

declare const process: { cwd(): string };

function errorMessage(action: () => unknown): string {
  try {
    action();
  } catch (error) {
    expect(error).toBeInstanceOf(DocAuditTypeScriptError);
    return (error as Error).message;
  }
  throw new Error('expected resolver to reject');
}

describe('TypeScript source-symbol resolver', () => {
  it('resolves exported and non-exported top-level declarations', () => {
    const source = `
      export function Exported() {}
      class LocalClass {}
      interface LocalInterface { value: string }
      type LocalType = string
      enum LocalEnum { One }
      const OtherValue = 0, LocalValue = 1
    `;
    expect(resolveTypeScriptSourceText(source, 'Exported')).toEqual({ kind: 'function', module: '<fixture>.ts', symbol: 'Exported' });
    expect(resolveTypeScriptSourceText(source, 'LocalClass')).toMatchObject({ kind: 'class' });
    expect(resolveTypeScriptSourceText(source, 'LocalInterface')).toMatchObject({ kind: 'interface' });
    expect(resolveTypeScriptSourceText(source, 'LocalType')).toMatchObject({ kind: 'type' });
    expect(resolveTypeScriptSourceText(source, 'LocalEnum')).toMatchObject({ kind: 'enum' });
    expect(resolveTypeScriptSourceText(source, 'LocalValue')).toMatchObject({ kind: 'variable' });
  });

  it('resolves a named top-level TSX declaration and ignores nested declarations', () => {
    const source = `
      export function Root() {
        function Nested() {}
        return <button onClick={Nested}>Done</button>;
      }
    `;
    expect(resolveTypeScriptSourceText(source, 'Root', 'fixture.tsx').kind).toBe('function');
    expect(errorMessage(() => resolveTypeScriptSourceText(source, 'Nested', 'fixture.tsx')))
      .toContain('dangling');
  });

  it('rejects missing and duplicate declarations', () => {
    const source = `
      function Duplicate() {}
      export function Duplicate() {}
    `;
    expect(errorMessage(() => resolveTypeScriptSourceText(source, 'Missing')))
      .toContain('dangling');
    expect(errorMessage(() => resolveTypeScriptSourceText(source, 'Duplicate')))
      .toContain('ambiguous');
  });

  it('rejects malformed and repository-escaping anchors', () => {
    const root = process.cwd().replace(/\/web$/, '');
    for (const anchor of [
      'fixture.ts#Root',
      'ts:fixture.ts',
      'ts:fixture.ts#',
      'ts:fixture.ts#not-a-symbol',
      'ts:../fixture.ts#Root',
      'ts:/tmp/fixture.ts#Root',
    ]) {
      expect(errorMessage(() => resolveTypeScriptAnchor(root, anchor))).toMatch(/malformed|escaping|dangling/);
    }
    expect(() => parseTypeScriptAnchor('ts:fixture.ts#Root#Other')).toThrow(/malformed/);
  });

  it('rejects anonymous declarations', () => {
    expect(errorMessage(() => resolveTypeScriptSourceText('export default function () {}\n', 'default')))
      .toContain('dangling');
  });

  it('rejects malformed command-line argument sequences', () => {
    expect(() => parseArguments(['--anchor', 'ts:fixture.ts#Root'])).toThrow(/usage/);
    expect(() => parseArguments(['--root', '/tmp', '--anchor'])).toThrow(/usage/);
    expect(() => parseArguments(['--root', '/tmp', '--unknown', 'value'])).toThrow(/arguments/);
  });

  it('keeps deterministic lexical anchor sorting', () => {
    const root = process.cwd().replace(/\/web$/, '');
    const result = resolveTypeScriptAnchors(root, [
      'ts:web/src/panes.ts#DEFAULT_WIDTHS',
      'ts:web/src/organizer.ts#notebookName',
    ]);
    expect(result.map((entry: { anchor: string }) => entry.anchor)).toEqual([
      'ts:web/src/organizer.ts#notebookName',
      'ts:web/src/panes.ts#DEFAULT_WIDTHS',
    ]);
  });

  it('resolves the real EditorPane and SyncCenter component anchors', () => {
    const root = process.cwd().replace(/\/web$/, '');
    expect(resolveTypeScriptAnchor(root, 'ts:web/src/App.tsx#App'))
      .toMatchObject({ kind: 'function', module: 'web/src/App.tsx', symbol: 'App' });
    expect(resolveTypeScriptAnchor(root, 'ts:web/src/components/EditorPane.tsx#EditorPane'))
      .toMatchObject({ kind: 'function', module: 'web/src/components/EditorPane.tsx', symbol: 'EditorPane' });
    expect(resolveTypeScriptAnchor(root, 'ts:web/src/components/SyncCenter.tsx#SyncCenter'))
      .toMatchObject({ kind: 'function', module: 'web/src/components/SyncCenter.tsx', symbol: 'SyncCenter' });
  });
});
