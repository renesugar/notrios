// The local graph: what surrounds the note in the editor.
//
// This is deliberately *not* a canvas. Notrios' target scale is tens of
// thousands of notes, where a global force-directed graph becomes an unreadable
// hairball — the failure mode Obsidian's global graph is known for, while its
// *local* graph stays useful at any size. So the question this answers is "what
// is next to this note", and the answer is a short grouped list rather than a
// picture.
//
// No graph library is involved, and none is needed: at depth 1–2 a
// neighbourhood is tens of nodes, and the traversal, the bounding, and the
// ordering are already done by the service.
import { useEffect, useState } from 'react';
import { getLocalGraph, LOCAL_GRAPH_DEPTHS, type GraphNode, type GraphResponse } from '../api';

export interface LocalGraphProps {
  documentID: string;
  onOpenDocument: (documentID: string) => void;
}

/** Groups neighbours by hop distance, dropping the root and sorting by label. */
export function neighboursByDepth(graph: GraphResponse, rootID: string): Map<number, GraphNode[]> {
  const grouped = new Map<number, GraphNode[]>();
  for (const node of graph.nodes) {
    if (node.id === rootID || node.depth === 0) continue;
    const bucket = grouped.get(node.depth) ?? [];
    bucket.push(node);
    grouped.set(node.depth, bucket);
  }
  for (const bucket of grouped.values()) {
    bucket.sort((a, b) => (a.label ?? a.id).localeCompare(b.label ?? b.id));
  }
  return new Map([...grouped.entries()].sort((a, b) => a[0] - b[0]));
}

/**
 * Says what a ceiling did, when one applied.
 *
 * A partial neighbourhood shown as if it were complete is worse than no
 * neighbourhood: it invites the conclusion that nothing else links here.
 */
export function describeGraphLimits(graph: GraphResponse): string | null {
  if (!graph.truncated) return null;
  const limit = graph.truncated_by === 'edges' ? 'links' : 'notes';
  return `Showing part of the neighbourhood — the ${limit} limit was reached, so more notes connect to this one.`;
}

export function LocalGraph({ documentID, onOpenDocument }: LocalGraphProps) {
  const [depth, setDepth] = useState<number>(1);
  const [graph, setGraph] = useState<GraphResponse | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    const controller = new AbortController();
    setGraph(null);
    setError('');
    getLocalGraph(documentID, depth, controller.signal)
      .then(setGraph)
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setError(cause instanceof Error ? cause.message : 'the neighbourhood could not be loaded');
      });
    return () => controller.abort();
  }, [documentID, depth]);

  const grouped = graph ? neighboursByDepth(graph, documentID) : new Map<number, GraphNode[]>();
  const total = [...grouped.values()].reduce((sum, bucket) => sum + bucket.length, 0);
  const limits = graph ? describeGraphLimits(graph) : null;

  return (
    <div className="local-graph" data-testid="local-graph">
      <div className="local-graph-header">
        <strong>Nearby notes</strong>
        <label className="local-graph-depth">
          Hops
          <select
            value={depth}
            aria-label="Hops from this note"
            onChange={(event) => setDepth(Number(event.target.value))}
          >
            {LOCAL_GRAPH_DEPTHS.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
      </div>

      {error && (
        <p className="muted" data-testid="local-graph-error">
          {error}
        </p>
      )}
      {!error && graph === null && <p className="muted">Loading…</p>}
      {!error && graph !== null && total === 0 && (
        <p className="muted" data-testid="local-graph-empty">
          Nothing links to or from this note{depth > 1 ? ` within ${depth} hops` : ''}.
        </p>
      )}

      {[...grouped.entries()].map(([hops, nodes]) => (
        <div key={hops} className="local-graph-group" data-testid={`local-graph-depth-${hops}`}>
          <span className="muted">
            {hops === 1 ? 'Directly linked' : `${hops} hops away`} · {nodes.length}
          </span>
          <ul>
            {nodes.map((node) => (
              <li key={node.id}>
                {node.kind === 'document' ? (
                  <button type="button" onClick={() => onOpenDocument(node.id)}>
                    {node.label || node.id}
                  </button>
                ) : (
                  // A resource or an unresolved target is a leaf, not somewhere
                  // to navigate to. Rendering it as a button that opens nothing
                  // would be a lie about what is there.
                  <span>
                    {node.label || node.id} <span className="muted">· {node.kind}</span>
                  </span>
                )}
              </li>
            ))}
          </ul>
        </div>
      ))}

      {limits && (
        <p className="muted" data-testid="local-graph-truncated">
          {limits}
        </p>
      )}
    </div>
  );
}
