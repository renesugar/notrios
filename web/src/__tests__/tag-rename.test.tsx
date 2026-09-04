// Renaming a tag hierarchy: the report comes first, and it has to be a report
// of the rename that is about to happen.
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TagRename } from '../components/TagRename';
import * as api from '../api';
import type { TagRenameResult } from '../api';

function result(overrides: Partial<TagRenameResult> = {}): TagRenameResult {
  return {
    from: 'field', to: 'fieldwork', include_children: true, dry_run: true,
    changes: [
      { tag_id: 't1', from: 'field', to: 'fieldwork', action: 'rename', notes: 4, notes_gained: 4 },
      { tag_id: 't2', from: 'field/dusk', to: 'fieldwork/dusk', action: 'rename', notes: 2, notes_gained: 2 },
    ],
    notes: 5,
    warnings: [],
    ...overrides,
  };
}

afterEach(() => vi.restoreAllMocks());

describe('renaming a tag', () => {
  it('cannot be applied before it has been previewed', async () => {
    render(<TagRename tag="field" onClose={vi.fn()} onRenamed={vi.fn()} />);
    await userEvent.clear(screen.getByTestId('tag-rename-to'));
    await userEvent.type(screen.getByTestId('tag-rename-to'), 'fieldwork');
    expect(screen.getByTestId('tag-rename-apply')).toBeDisabled();
  });

  it('shows what the rename would touch before doing it', async () => {
    const rename = vi.spyOn(api, 'renameTag').mockResolvedValue(result());
    render(<TagRename tag="field" onClose={vi.fn()} onRenamed={vi.fn()} />);
    await userEvent.clear(screen.getByTestId('tag-rename-to'));
    await userEvent.type(screen.getByTestId('tag-rename-to'), 'fieldwork');
    await userEvent.click(screen.getByTestId('tag-rename-preview'));
    // Dry run, explicitly, rather than relying on the service's default.
    expect(rename).toHaveBeenCalledExactlyOnceWith('field', 'fieldwork', true, true);
    const report = await screen.findByTestId('tag-rename-report');
    expect(report).toHaveTextContent('2 tags affected');
    expect(report).toHaveTextContent('5 notes');
  });

  // The destination already exists, and afterwards the two tags are one. A
  // reader expecting a rename should not have to infer that from a count.
  it('says when a change is a merge, and how many notes actually gain the tag', async () => {
    vi.spyOn(api, 'renameTag').mockResolvedValue(result({
      changes: [{ tag_id: 't1', from: 'field', to: 'work', action: 'merge',
                  merged_into_tag_id: 't9', notes: 6, notes_gained: 2 }],
    }));
    render(<TagRename tag="field" onClose={vi.fn()} onRenamed={vi.fn()} />);
    await userEvent.clear(screen.getByTestId('tag-rename-to'));
    await userEvent.type(screen.getByTestId('tag-rename-to'), 'work');
    await userEvent.click(screen.getByTestId('tag-rename-preview'));
    const report = await screen.findByTestId('tag-rename-report');
    expect(report).toHaveTextContent('merges into an existing tag');
    expect(report).toHaveTextContent('2 of');
  });

  it('surfaces the warnings rather than dropping them', async () => {
    vi.spyOn(api, 'renameTag').mockResolvedValue(result({
      warnings: ['the saved search "Dusk field notes" still mentions field'],
    }));
    render(<TagRename tag="field" onClose={vi.fn()} onRenamed={vi.fn()} />);
    await userEvent.clear(screen.getByTestId('tag-rename-to'));
    await userEvent.type(screen.getByTestId('tag-rename-to'), 'fieldwork');
    await userEvent.click(screen.getByTestId('tag-rename-preview'));
    expect(await screen.findByTestId('tag-rename-warnings'))
      .toHaveTextContent('still mentions field');
  });

  // Agreeing to a report of one rename and applying a different one is the
  // failure this whole shape exists to prevent.
  it('withdraws the report when the rename is edited', async () => {
    vi.spyOn(api, 'renameTag').mockResolvedValue(result());
    render(<TagRename tag="field" onClose={vi.fn()} onRenamed={vi.fn()} />);
    await userEvent.clear(screen.getByTestId('tag-rename-to'));
    await userEvent.type(screen.getByTestId('tag-rename-to'), 'fieldwork');
    await userEvent.click(screen.getByTestId('tag-rename-preview'));
    await screen.findByTestId('tag-rename-report');

    await userEvent.type(screen.getByTestId('tag-rename-to'), 'ing');
    expect(screen.queryByTestId('tag-rename-report')).toBeNull();
    expect(screen.getByTestId('tag-rename-apply')).toBeDisabled();
  });

  it('withdraws the report when the children choice changes', async () => {
    vi.spyOn(api, 'renameTag').mockResolvedValue(result());
    render(<TagRename tag="field" onClose={vi.fn()} onRenamed={vi.fn()} />);
    await userEvent.clear(screen.getByTestId('tag-rename-to'));
    await userEvent.type(screen.getByTestId('tag-rename-to'), 'fieldwork');
    await userEvent.click(screen.getByTestId('tag-rename-preview'));
    await screen.findByTestId('tag-rename-report');

    await userEvent.click(screen.getByTestId('tag-rename-children'));
    expect(screen.queryByTestId('tag-rename-report')).toBeNull();
  });

  it('applies the rename it previewed', async () => {
    const rename = vi.spyOn(api, 'renameTag').mockResolvedValue(result());
    const onRenamed = vi.fn();
    render(<TagRename tag="field" onClose={vi.fn()} onRenamed={onRenamed} />);
    await userEvent.clear(screen.getByTestId('tag-rename-to'));
    await userEvent.type(screen.getByTestId('tag-rename-to'), 'fieldwork');
    await userEvent.click(screen.getByTestId('tag-rename-preview'));
    await screen.findByTestId('tag-rename-report');
    await userEvent.click(screen.getByTestId('tag-rename-apply'));
    expect(rename).toHaveBeenLastCalledWith('field', 'fieldwork', true, false);
    expect(onRenamed).toHaveBeenCalledOnce();
  });

  it('reports a refusal against the dialog', async () => {
    vi.spyOn(api, 'renameTag').mockRejectedValue(new Error('cannot rename "field" into its own subtree'));
    render(<TagRename tag="field" onClose={vi.fn()} onRenamed={vi.fn()} />);
    await userEvent.clear(screen.getByTestId('tag-rename-to'));
    await userEvent.type(screen.getByTestId('tag-rename-to'), 'field/work');
    await userEvent.click(screen.getByTestId('tag-rename-preview'));
    expect(await screen.findByTestId('tag-rename-error')).toHaveTextContent('own subtree');
  });
});
