// Sidebar pane: search notebooks and the notebook tree (composed with the
// deterministic ordering in sidebar.ts) above the tag list with counts.
import type { TagRecord } from '../api';
import { isDeletableNotebookRow, type SidebarRow } from '../sidebar';

export interface SidebarPaneProps {
  rows: SidebarRow[];
  tags: TagRecord[];
  activeQuery: string;
  /**
   * The selected row's ID. Selection is tracked by ID rather than by the row's
   * query because a notebook row's query is `notebook:"<name>"` and notebook
   * names are unique only among siblings — `Contacts/Work` and `Personal/Work`
   * produce the same query, and highlighting by query would light up both.
   */
  selectedRowID: string | null;
  onSelectRow: (row: SidebarRow) => void;
  onSelectQuery: (query: string) => void;
  /** Delete a notebook; the caller previews and confirms before doing it. */
  onDeleteNotebook: (row: SidebarRow) => void;
}

export function SidebarPane({ rows, tags, activeQuery, selectedRowID, onSelectRow, onSelectQuery, onDeleteNotebook }: SidebarPaneProps) {
  return (
    <nav className="pane sidebar-pane" aria-label="Notebooks and tags" data-testid="pane-sidebar">
      <div className="pane-scroll">
        <h3>Notebooks</h3>
        <ul className="sidebar-list">
          {rows.map((row) => (
            <li key={row.id} className="sidebar-row">
              <button
                type="button"
                className={selectedRowID === row.id ? 'sidebar-item active' : 'sidebar-item'}
                style={row.depth > 0 ? { paddingLeft: `${12 + row.depth * 16}px` } : undefined}
                aria-current={selectedRowID === row.id ? 'true' : undefined}
                data-testid={`sidebar-row-${row.id}`}
                onClick={() => onSelectRow(row)}
              >
                <span className="sidebar-label">
                  {row.emoji ? `${row.emoji} ` : ''}
                  {row.label}
                </span>
              </button>
              {isDeletableNotebookRow(row) && (
                <button
                  type="button"
                  className="sidebar-action"
                  data-testid={`sidebar-delete-${row.id}`}
                  title={`Delete the notebook “${row.label}”`}
                  aria-label={`Delete the notebook ${row.label}`}
                  onClick={() => onDeleteNotebook(row)}
                >
                  🗑
                </button>
              )}
            </li>
          ))}
        </ul>
        {tags.length > 0 && (
          <>
            <h3>Tags</h3>
            <ul className="sidebar-list" data-testid="sidebar-tags">
              {tags.map((tag) => (
                <li key={tag.id}>
                  <button
                    type="button"
                    className={activeQuery === `tag:"${tag.name}"` ? 'sidebar-item active' : 'sidebar-item'}
                    aria-current={activeQuery === `tag:"${tag.name}"` ? 'true' : undefined}
                    onClick={() => onSelectQuery(`tag:"${tag.name}"`)}
                  >
                    <span className="sidebar-label">{tag.name}</span>
                    <span className="tag-count">{tag.note_count}</span>
                  </button>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
    </nav>
  );
}
