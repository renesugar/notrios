// Sidebar pane: search notebooks and the notebook tree (composed with the
// deterministic ordering in sidebar.ts) above the tag list with counts.
import type { TagRecord } from '../api';
import type { SidebarRow } from '../sidebar';

export interface SidebarPaneProps {
  rows: SidebarRow[];
  tags: TagRecord[];
  activeQuery: string;
  onSelectQuery: (query: string) => void;
}

export function SidebarPane({ rows, tags, activeQuery, onSelectQuery }: SidebarPaneProps) {
  return (
    <nav className="pane sidebar-pane" aria-label="Notebooks and tags" data-testid="pane-sidebar">
      <div className="pane-scroll">
        <h3>Notebooks</h3>
        <ul className="sidebar-list">
          {rows.map((row) => (
            <li key={row.id}>
              <button
                type="button"
                className={activeQuery === row.query ? 'sidebar-item active' : 'sidebar-item'}
                style={row.depth > 0 ? { paddingLeft: `${12 + row.depth * 16}px` } : undefined}
                aria-current={activeQuery === row.query ? 'true' : undefined}
                data-testid={`sidebar-row-${row.id}`}
                onClick={() => onSelectQuery(row.query)}
              >
                <span className="sidebar-label">
                  {row.emoji ? `${row.emoji} ` : ''}
                  {row.label}
                </span>
              </button>
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
