import { render, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PreviewPane } from '../components/PreviewPane';

// What the preview shows of a link that will not open, before J40-B (v1.0
// J40-A). The renderer has no wiki-link rule, so `[[name]]` is prose: its
// brackets are printed and nothing marks it. A Markdown link whose target is
// not a scheme the preview routes keeps its anchor and loses its href, so it is
// an `<a>` that goes nowhere and looks like any other link. Neither says why,
// and the preview never asks the service: it renders the body alone.
describe('J40-A: the preview before ambiguous links are marked', () => {
  it('prints a wiki link as text and keeps a dead Markdown link as an anchor with no href', async () => {
    const { container } = render(
      <PreviewPane
        body={'Wiki [[index]] here.\n\nMarkdown [shown](index.md) here.\n\nWorking [ok](document://default/documents/doc_ok).\n'}
        themeBase="light"
        onOpenDocument={() => {}}
        onOpenStableLink={() => {}}
        onError={() => {}}
      />,
    );
    await waitFor(() => {
      expect(container.textContent).toContain('Wiki [[index]] here.');
    });
    const anchors = Array.from(container.querySelectorAll('#notrios-preview a'));
    const dead = anchors.find((anchor) => anchor.textContent === 'shown');
    const working = anchors.find((anchor) => anchor.textContent === 'ok');
    expect(dead).toBeDefined();
    expect(dead!.hasAttribute('href')).toBe(false);
    expect(dead!.className).toBe(working!.className);
    expect(dead!.hasAttribute('title')).toBe(false);
    // Nothing in the wiki link's place is an element of its own.
    expect(anchors.some((anchor) => anchor.textContent?.includes('index') && anchor !== dead)).toBe(false);
  });
});
