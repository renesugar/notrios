// Publishing: the review is the feature, and the digest binds it to the run.
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { PublishPanel, type PublishBridge } from '../components/PublishPanel';

const plan = {
  profile: 'blog', target: 'publication_handoff',
  manifest_sha256: 'a'.repeat(64),
  counts: {
    selected_documents: 12, excluded_documents: 40, private_links: 3,
    broken_links: 0, reachable_resources: 5, oversized_resources: 1,
  },
  exclusions: [{ kind: 'document', id: 'doc_9', reason: 'carries a private tag' }],
  warnings: ['one note links to a withheld note and will be rewritten'],
  truncated: false,
  command: 'notriosctl publish run --profile "blog" --reviewed-plan ' + 'a'.repeat(64) + ' <out-dir>',
};

function bridge(overrides: Partial<PublishBridge> = {}): PublishBridge {
  return {
    ChooseDirectory: vi.fn(async () => '/tmp/out'),
    PublishProfiles: vi.fn(async () => [{ name: 'blog', description: 'public notes', target: 'publication_handoff' }]),
    PlanPublication: vi.fn(async () => plan),
    Publish: vi.fn(async () => ({
      profile: 'blog', directory: '/tmp/out', documents: 12, objects: 30, manifest_sha256: 'a'.repeat(64),
    })),
    ...overrides,
  };
}

describe('publishing', () => {
  it('explains rather than hides itself without the desktop bridge', () => {
    render(<PublishPanel bridge={undefined} />);
    expect(screen.getByTestId('publish-unavailable')).toHaveTextContent('notriosctl publish run');
  });

  // A Go nil slice crosses as null. Reading .length off it took the About
  // dialog down; the same shape would blank this panel.
  it('survives a null profile list', async () => {
    render(<PublishPanel bridge={bridge({ PublishProfiles: vi.fn(async () => null) })} />);
    expect(await screen.findByTestId('publish-no-profiles')).toBeInTheDocument();
  });

  it('sends somebody to the command line rather than offering a profile form', async () => {
    render(<PublishPanel bridge={bridge({ PublishProfiles: vi.fn(async () => []) })} />);
    expect(await screen.findByTestId('publish-no-profiles'))
      .toHaveTextContent('notriosctl publish profile save');
  });

  it('cannot publish before the review has been read', async () => {
    render(<PublishPanel bridge={bridge()} />);
    await userEvent.type(await screen.findByTestId('publish-directory'), '/tmp/out');
    expect(screen.getByTestId('publish-run')).toBeDisabled();
  });

  // What is withheld and what is rewritten matters more than what is sent:
  // publishing something private is the failure this screen exists to prevent.
  it('shows what is withheld and rewritten, not only what is sent', async () => {
    render(<PublishPanel bridge={bridge()} />);
    await userEvent.click(await screen.findByTestId('publish-review'));
    expect(await screen.findByTestId('publish-count-excluded_documents')).toHaveTextContent('40 · notes withheld');
    expect(screen.getByTestId('publish-count-private_links')).toHaveTextContent('3 · links to withheld notes');
    expect(screen.getByTestId('publish-count-oversized_resources')).toHaveTextContent('1 · attachments too large');
    expect(screen.getByTestId('publish-warnings')).toHaveTextContent('will be rewritten');
  });

  it('publishes the digest it reviewed, not the profile name alone', async () => {
    const bound = bridge();
    render(<PublishPanel bridge={bound} />);
    await userEvent.click(await screen.findByTestId('publish-review'));
    await screen.findByTestId('publish-plan');
    await userEvent.type(screen.getByTestId('publish-directory'), '/tmp/out');
    await userEvent.click(screen.getByTestId('publish-run'));
    expect(bound.Publish).toHaveBeenCalledExactlyOnceWith('blog', 'a'.repeat(64), '/tmp/out');
    expect(await screen.findByTestId('publish-result')).toHaveTextContent('Published 12 notes');
  });

  // A digest belongs to one profile. Reviewing one and publishing another is
  // precisely what the reviewed-plan check exists to stop.
  it('withdraws the review when the profile changes', async () => {
    render(<PublishPanel bridge={bridge({
      PublishProfiles: vi.fn(async () => [
        { name: 'blog', description: '', target: 'publication_handoff' },
        { name: 'talk', description: '', target: 'publication_handoff' },
      ]),
    })} />);
    await userEvent.selectOptions(await screen.findByTestId('publish-profile'), 'blog');
    await userEvent.click(screen.getByTestId('publish-review'));
    await screen.findByTestId('publish-plan');
    await userEvent.selectOptions(screen.getByTestId('publish-profile'), 'talk');
    expect(screen.queryByTestId('publish-plan')).toBeNull();
    expect(screen.getByTestId('publish-run')).toBeDisabled();
  });

  it('reports a refused publication against the panel', async () => {
    render(<PublishPanel bridge={bridge({
      Publish: vi.fn(async () => { throw new Error('the library changed since the plan was reviewed'); }),
    })} />);
    await userEvent.click(await screen.findByTestId('publish-review'));
    await screen.findByTestId('publish-plan');
    await userEvent.type(screen.getByTestId('publish-directory'), '/tmp/out');
    await userEvent.click(screen.getByTestId('publish-run'));
    expect(await screen.findByTestId('publish-error')).toHaveTextContent('changed since the plan was reviewed');
  });
});
