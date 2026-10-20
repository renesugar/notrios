import { render, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PreviewPane } from '../components/PreviewPane';

// KaTeX is the preview's math renderer, handed to md-editor-rt as an instance
// by installEditorAssets. A KaTeX upgrade that changed what md-editor-rt calls
// would show the TeX source instead of math, so this renders it through the
// same pane, sanitiser and instance the app uses (v1.0 J45).
describe('preview math', () => {
  it('renders inline and display math with the bundled KaTeX', async () => {
    const { container } = render(
      <PreviewPane
        body={'Inline $x^2$ here.\n\n$$\n\\frac{a}{b}\n$$\n'}
        themeBase="light"
        onOpenDocument={() => {}}
        onOpenStableLink={() => {}}
        onError={() => {}}
      />,
    );
    await waitFor(() => {
      expect(container.querySelectorAll('.katex').length).toBeGreaterThanOrEqual(2);
    });
    expect(container.querySelector('.katex-display .mfrac')).not.toBeNull();
    expect(container.querySelector('.katex-error')).toBeNull();
  });
});
