// The local graph, which is F5's answer to "a graph view" at a scale where a
// global canvas stops being readable.
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { LocalGraph, describeGraphLimits, neighboursByDepth } from '../components/LocalGraph';
import type { GraphResponse } from '../api';

function graph(partial: Partial<GraphResponse> = {}): GraphResponse {
  return {
    nodes: [],
    edges: [],
    requested_depth: 1,
    completed_depth: 1,
    ...partial,
  };
}

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('neighboursByDepth', () => {
  it('drops the note itself and groups the rest by hops', () => {
    const grouped = neighboursByDepth(
      graph({
        nodes: [
          { id: 'root', kind: 'document', label: 'Root', depth: 0 },
          { id: 'b', kind: 'document', label: 'Beta', depth: 1 },
          { id: 'a', kind: 'document', label: 'Alpha', depth: 1 },
          { id: 'c', kind: 'document', label: 'Gamma', depth: 2 },
        ],
      }),
      'root',
    );
    expect([...grouped.keys()]).toEqual([1, 2]);
    // Sorted by label, so the list does not reorder itself between reads.
    expect(grouped.get(1)?.map((node) => node.id)).toEqual(['a', 'b']);
    expect(grouped.get(2)?.map((node) => node.id)).toEqual(['c']);
  });
});

describe('describeGraphLimits', () => {
  // A partial neighbourhood shown as a complete one invites exactly the wrong
  // conclusion: that nothing else links here.
  it('says nothing when the answer is complete', () => {
    expect(describeGraphLimits(graph())).toBeNull();
  });

  it('names which ceiling stopped the expansion', () => {
    expect(describeGraphLimits(graph({ truncated: true, truncated_by: 'nodes' }))).toContain('notes limit');
    expect(describeGraphLimits(graph({ truncated: true, truncated_by: 'edges' }))).toContain('links limit');
  });
});

describe('LocalGraph', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('asks for the neighbourhood of the open note and lists it by hops', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(
        graph({
          nodes: [
            { id: 'doc_root', kind: 'document', label: 'Root', depth: 0 },
            { id: 'doc_near', kind: 'document', label: 'Kitchen Plan', depth: 1 },
          ],
        }),
      ),
    );
    vi.stubGlobal('fetch', fetchMock);
    render(<LocalGraph documentID="doc_root" onOpenDocument={() => {}} />);

    await waitFor(() => expect(screen.getByText('Kitchen Plan')).toBeTruthy());
    const [, init] = fetchMock.mock.calls[0];
    expect(JSON.parse(init.body)).toEqual({ roots: ['doc_root'], depth: 1, direction: 'both' });
    // The note itself is not listed as its own neighbour.
    expect(screen.queryByText('Root')).toBeNull();
  });

  it('re-asks at the new depth when the hop count changes', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(graph()));
    vi.stubGlobal('fetch', fetchMock);
    const user = userEvent.setup();
    render(<LocalGraph documentID="doc_root" onOpenDocument={() => {}} />);

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    await user.selectOptions(screen.getByLabelText('Hops from this note'), '2');
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(JSON.parse(fetchMock.mock.calls[1][1].body).depth).toBe(2);
  });

  it('opens a neighbour but offers no such affordance for a leaf', async () => {
    const opened: string[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(
          graph({
            nodes: [
              { id: 'doc_near', kind: 'document', label: 'Kitchen Plan', depth: 1 },
              { id: 'res_pic', kind: 'resource', label: 'pic.png', depth: 1 },
              { id: 'Kitchen', kind: 'unresolved', label: 'Kitchen', depth: 1 },
            ],
          }),
        ),
      ),
    );
    const user = userEvent.setup();
    render(<LocalGraph documentID="doc_root" onOpenDocument={(id) => opened.push(id)} />);

    await waitFor(() => expect(screen.getByText('Kitchen Plan')).toBeTruthy());
    await user.click(screen.getByText('Kitchen Plan'));
    expect(opened).toEqual(['doc_near']);

    // A resource and a broken target are leaves. A button that opens nothing
    // would be a lie about what is there.
    expect(screen.getAllByRole('button').map((button) => button.textContent)).toEqual(['Kitchen Plan']);
  });

  it('says so plainly when nothing links to the note', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(graph())));
    render(<LocalGraph documentID="doc_lonely" onOpenDocument={() => {}} />);
    await waitFor(() => expect(screen.getByTestId('local-graph-empty')).toBeTruthy());
  });

  it('reports a failed load rather than rendering an empty neighbourhood', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('service unreachable')));
    render(<LocalGraph documentID="doc_root" onOpenDocument={() => {}} />);
    await waitFor(() => expect(screen.getByTestId('local-graph-error')).toBeTruthy());
    // "Nothing links here" and "we could not ask" are different facts.
    expect(screen.queryByTestId('local-graph-empty')).toBeNull();
  });
});
