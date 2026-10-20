import { describe, expect, it } from 'vitest';
// package.json overrides mermaid's elkjs with web/elk-refused, so this is the
// stub mermaid gets, not elkjs.
import ELK, { ELK_REFUSAL } from 'elkjs/lib/elk.bundled.js';

const installed = import.meta.glob('/node_modules/elkjs/package.json', {
  import: 'default',
  eager: true,
}) as Record<string, { name: string; license: string; version: string }>;

// The chunk mermaid loads for an ELK layout, read as text, so a mermaid that
// imports elkjs under another path fails here instead of reaching past the stub.
const elkChunks = import.meta.glob('/node_modules/mermaid/dist/chunks/mermaid.core/elk-*.mjs', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>;

describe('ELK stays out of the install and the build (v1.0 J45)', () => {
  it('installs the stub, not elkjs', () => {
    const manifests = Object.values(installed);
    expect(manifests).toHaveLength(1);
    expect(manifests[0]).toMatchObject({ name: 'elkjs', version: '0.0.0-notrios', license: 'Apache-2.0' });
  });

  it('refuses every layout', async () => {
    await expect(new ELK().layout({ id: 'root' })).rejects.toThrow(ELK_REFUSAL);
  });

  it('is the only path mermaid imports elkjs by', () => {
    const sources = Object.values(elkChunks);
    expect(sources.length).toBeGreaterThan(0);
    const imports = sources.flatMap((source) =>
      [...source.matchAll(/from\s+["'](elkjs[^"']*)["']/g)].map((match) => match[1]),
    );
    expect(imports.length).toBeGreaterThan(0);
    expect(new Set(imports)).toEqual(new Set(['elkjs/lib/elk.bundled.js']));
  });
});
