// Keeping a search as a notebook: an action on a search that ran, not a form.
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { SearchPane, type SearchPaneProps } from '../components/SearchPane';
import type { SearchHit } from '../api';

const hit = { id: 'doc_1', uri: 'x', title: 'Reed beds at dusk' } as SearchHit;

function props(overrides: Partial<SearchPaneProps> = {}): SearchPaneProps {
  return {
    query: '',
    onQueryChange: vi.fn(),
    onSubmit: vi.fn(),
    paged: {
      hits: [hit], loading: false, exhausted: true, started: true, error: null,
      start: vi.fn(), loadMore: vi.fn(), patchHit: vi.fn(), prependHit: vi.fn(), removeHit: vi.fn(),
    },
    onOpenHit: vi.fn(),
    selectedDocumentID: null,
    checked: new Set<string>(),
    onToggleChecked: vi.fn(),
    busy: false,
    onNewNote: vi.fn(),
    newNoteNotebookName: 'Notes',
    ranQuery: 'tag:field/dusk -notebook:"Help"',
    onKeepSearch: vi.fn(async () => undefined),
    ...overrides,
  };
}

describe('keeping a search', () => {
  it('is not offered before a search has run', () => {
    render(<SearchPane {...props({
      paged: { ...props().paged, started: false },
    })} />);
    expect(screen.queryByTestId('keep-search')).toBeNull();
  });

  // A notebook made from a query nobody has seen work is a view that may be
  // empty, misspelled, or matching something other than what was meant.
  it('is not offered for a search that found nothing', () => {
    render(<SearchPane {...props({ paged: { ...props().paged, hits: [] } })} />);
    expect(screen.queryByTestId('keep-search')).toBeNull();
  });

  it('keeps the query that ran, not what is now in the box', async () => {
    const onKeepSearch = vi.fn(async () => undefined);
    // The box has been typed into since the search ran, which is what people
    // do while reading results.
    render(<SearchPane {...props({ query: 'half-typed something else', onKeepSearch })} />);
    await userEvent.click(screen.getByTestId('keep-search'));
    expect(screen.getByTestId('keep-search-query')).toHaveTextContent('tag:field/dusk -notebook:"Help"');
    await userEvent.click(screen.getByTestId('keep-search-save'));
    expect(onKeepSearch).toHaveBeenCalledOnce();
  });

  it('suggests the query as the name and lets it be changed', async () => {
    const onKeepSearch = vi.fn(async () => undefined);
    render(<SearchPane {...props({ onKeepSearch })} />);
    await userEvent.click(screen.getByTestId('keep-search'));
    const name = screen.getByTestId('keep-search-name');
    expect(name).toHaveValue('tag:field/dusk -notebook:"Help"');
    await userEvent.clear(name);
    await userEvent.type(name, 'Dusk field notes');
    await userEvent.click(screen.getByTestId('keep-search-save'));
    expect(onKeepSearch).toHaveBeenCalledExactlyOnceWith('Dusk field notes');
  });

  it('refuses to keep a search under no name', async () => {
    render(<SearchPane {...props()} />);
    await userEvent.click(screen.getByTestId('keep-search'));
    await userEvent.clear(screen.getByTestId('keep-search-name'));
    expect(screen.getByTestId('keep-search-save')).toBeDisabled();
  });

  // A name conflict is the common failure and belongs beside the field, not at
  // the top of the window where the field is no longer in view.
  it('reports a refusal against the form', async () => {
    const onKeepSearch = vi.fn(async () => { throw new Error('a notebook called that already exists'); });
    render(<SearchPane {...props({ onKeepSearch })} />);
    await userEvent.click(screen.getByTestId('keep-search'));
    await userEvent.click(screen.getByTestId('keep-search-save'));
    expect(await screen.findByTestId('keep-search-error')).toHaveTextContent('already exists');
    expect(screen.getByTestId('keep-search-form')).toBeInTheDocument();
  });
});
