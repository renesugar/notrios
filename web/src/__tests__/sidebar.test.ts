// Sidebar ordering invariants (UI_DESIGN.md / NOTEBOOKS_AND_SEARCH_NOTEBOOKS.md):
// All notes first, Trash last, the read-only builtins (Reports, then Help)
// immediately above Trash, user content in between, ordering driven by stable
// builtin IDs — never by names or response order.
import { describe, expect, it } from 'vitest';
import {
  composeSidebar,
  isDeletableNotebookRow,
  ALL_NOTES_SEARCH_ID,
  DEFAULT_NOTEBOOK_ID,
  HELP_NOTEBOOK_ID,
  REPORTS_NOTEBOOK_ID,
  TRASH_SEARCH_ID,
} from '../sidebar';
import type { NotebookTreeNode, SearchNotebook } from '../api';

function nb(id: string, name: string, extra: Partial<NotebookTreeNode> = {}): NotebookTreeNode {
  return { id, name, builtin: false, position: 0, ...extra };
}

function sn(id: string, name: string, anchor: string, extra: Partial<SearchNotebook> = {}): SearchNotebook {
  return { id, name, query: extra.query ?? `q:${id}`, builtin: id.startsWith('snb_'), sort_anchor: anchor, ...extra };
}

const allNotes = sn(ALL_NOTES_SEARCH_ID, 'All notes', 'first', { query: '' });
const trash = sn(TRASH_SEARCH_ID, 'Trash', 'last', { query: 'is:trashed' });
const help = nb(HELP_NOTEBOOK_ID, 'Help', { builtin: true });
const reports = nb(REPORTS_NOTEBOOK_ID, 'Reports', { builtin: true });

function ids(rows: ReturnType<typeof composeSidebar>) {
  return rows.map((row) => row.id);
}

describe('composeSidebar', () => {
  // F5 adds a second read-only builtin. The pair sits above Trash in a fixed
  // order, so a generated report has a stable home a reader can find.
  it('stacks Reports above Help above Trash', () => {
    const rows = composeSidebar([nb('nb_a', 'Alpha'), help, reports], [allNotes, trash]);
    const order = ids(rows);
    expect(order.at(-1)).toBe(TRASH_SEARCH_ID);
    expect(order.at(-2)).toBe(HELP_NOTEBOOK_ID);
    expect(order.at(-3)).toBe(REPORTS_NOTEBOOK_ID);
    // And neither appears inside the user's tree.
    expect(order.indexOf(REPORTS_NOTEBOOK_ID)).toBeGreaterThan(order.indexOf('nb_a'));
  });

  // An empty Reports notebook still shows: a builtin that appears only once it
  // has content is a builtin nobody discovers.
  it('shows Reports even when the tree is otherwise empty', () => {
    expect(ids(composeSidebar([reports, help], [allNotes, trash]))).toEqual([
      ALL_NOTES_SEARCH_ID,
      REPORTS_NOTEBOOK_ID,
      HELP_NOTEBOOK_ID,
      TRASH_SEARCH_ID,
    ]);
  });

  // The two sets the store keeps distinct, mirrored here: Notes is undeletable
  // without being read-only, and the read-only builtins are both.
  it('offers no delete affordance for Notes, Help or Reports', () => {
    const rows = composeSidebar([nb(DEFAULT_NOTEBOOK_ID, 'Notes'), nb('nb_a', 'Alpha'), reports, help], [allNotes, trash]);
    const deletable = rows.filter(isDeletableNotebookRow).map((row) => row.id);
    expect(deletable).toEqual(['nb_a']);
  });

  it('places All notes first, Trash last, Help immediately above Trash', () => {
    const rows = composeSidebar([nb('nb_a', 'Alpha'), help, nb('nb_z', 'Zulu')], [allNotes, trash]);
    const order = ids(rows);
    expect(order[0]).toBe(ALL_NOTES_SEARCH_ID);
    expect(order[order.length - 1]).toBe(TRASH_SEARCH_ID);
    expect(order[order.length - 2]).toBe(HELP_NOTEBOOK_ID);
  });

  it('keeps Help above Trash regardless of API response order', () => {
    // Trash first, All notes last, Help buried in the middle of the tree list.
    const rows = composeSidebar([nb('nb_z', 'Zulu'), help, nb('nb_a', 'Alpha')], [trash, sn('snb_user', 'TODO', 'normal'), allNotes]);
    const order = ids(rows);
    expect(order[0]).toBe(ALL_NOTES_SEARCH_ID);
    expect(order.at(-1)).toBe(TRASH_SEARCH_ID);
    expect(order.at(-2)).toBe(HELP_NOTEBOOK_ID);
  });

  it('is not displaced by names sorting before or after "Help"', () => {
    const rows = composeSidebar(
      [nb('nb_1', 'Aardvark'), nb('nb_2', 'Helq'), nb('nb_3', 'Hel'), nb('nb_4', 'Zz'), help],
      [allNotes, trash],
    );
    const order = ids(rows);
    expect(order.at(-2)).toBe(HELP_NOTEBOOK_ID);
    expect(order.at(-1)).toBe(TRASH_SEARCH_ID);
  });

  it('keeps user search notebooks visible, above Help and Trash', () => {
    const rows = composeSidebar([help, nb('nb_a', 'Alpha')], [allNotes, trash, sn('snb_todo', 'TODO', 'normal'), sn('snb_zzz', 'ZZZ', 'normal')]);
    const order = ids(rows);
    expect(order).toContain('snb_todo');
    expect(order.indexOf('snb_todo')).toBeLessThan(order.indexOf(HELP_NOTEBOOK_ID));
    expect(order.at(-2)).toBe(HELP_NOTEBOOK_ID);
    expect(order.at(-1)).toBe(TRASH_SEARCH_ID);
  });

  it('preserves nested notebook hierarchy with depths and varying positions', () => {
    const tree = [
      nb('nb_work', 'Work', { position: 5, children: [nb('nb_reports', 'Reports', { children: [nb('nb_q3', 'Q3')] })] }),
      help,
      nb('nb_misc', 'Misc', { position: 1 }),
    ];
    const rows = composeSidebar(tree, [allNotes, trash]);
    const work = rows.find((row) => row.id === 'nb_work');
    const reports = rows.find((row) => row.id === 'nb_reports');
    const q3 = rows.find((row) => row.id === 'nb_q3');
    expect(work?.depth).toBe(0);
    expect(reports?.depth).toBe(1);
    expect(q3?.depth).toBe(2);
    // Children immediately follow their parent.
    expect(ids(rows).indexOf('nb_reports')).toBe(ids(rows).indexOf('nb_work') + 1);
    expect(ids(rows).at(-2)).toBe(HELP_NOTEBOOK_ID);
  });

  it('handles unknown anchored builtins deliberately without breaking Help/Trash adjacency', () => {
    const rows = composeSidebar(
      [help, nb('nb_a', 'Alpha')],
      [allNotes, trash, sn('snb_pinned', 'Pinned', 'first'), sn('snb_archive', 'Archive', 'last')],
    );
    const order = ids(rows);
    expect(order[0]).toBe(ALL_NOTES_SEARCH_ID);
    expect(order[1]).toBe('snb_pinned'); // extra first-anchor follows All notes
    expect(order).toContain('snb_archive'); // extra last-anchor is kept…
    expect(order.at(-2)).toBe(HELP_NOTEBOOK_ID); // …but Help stays glued to Trash
    expect(order.at(-1)).toBe(TRASH_SEARCH_ID);
  });

  it('builds a notebook-scoped query for Help so activation searches the Help notebook', () => {
    const rows = composeSidebar([help], [allNotes, trash]);
    const helpRow = rows.find((row) => row.id === HELP_NOTEBOOK_ID);
    expect(helpRow?.query).toBe('notebook:"Help"');
  });
});
