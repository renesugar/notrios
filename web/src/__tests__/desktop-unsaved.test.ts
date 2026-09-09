// Telling the desktop shell that the editor is holding unsaved work.
//
// The window's close button does not go through `beforeunload`, so the shell
// asks in Go and can only do that if it has been told (see
// cmd/notrios/gui_window_state.go). The interesting case is the one that has
// already caused two defects in this frontend: Wails injects `window.go` after
// the webview starts, so a report sent at the moment the state changed can
// arrive before there is anything to receive it.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { reportUnsavedChanges } from '../desktop';

type BoundWindow = Window & { go?: { main?: { WindowState?: { SetUnsavedChanges: (unsaved: boolean) => Promise<void> } } } };

function bind(fn: (unsaved: boolean) => Promise<void>) {
  (window as BoundWindow).go = { main: { WindowState: { SetUnsavedChanges: fn } } };
}

beforeEach(() => {
  vi.useFakeTimers();
  delete (window as BoundWindow).go;
});

afterEach(() => {
  // Drain any retry loop this test left running, so it cannot outlive the test.
  vi.advanceTimersByTime(6000);
  vi.useRealTimers();
  delete (window as BoundWindow).go;
});

describe('reporting unsaved work', () => {
  it('sends both states, so the shell stops asking once the work is saved', async () => {
    const sent: boolean[] = [];
    bind((unsaved) => {
      sent.push(unsaved);
      return Promise.resolve();
    });

    reportUnsavedChanges(true);
    reportUnsavedChanges(false);
    expect(sent).toEqual([true, false]);
  });

  it('waits for the binding rather than dropping the report', () => {
    const sent: boolean[] = [];
    reportUnsavedChanges(true);
    expect(sent).toEqual([]);

    // The binding appears a moment later, as it does in the running app.
    bind((unsaved) => {
      sent.push(unsaved);
      return Promise.resolve();
    });
    vi.advanceTimersByTime(150);
    expect(sent).toEqual([true]);

    // And the retry loop stops once it has delivered: a later change is sent
    // straight away rather than waiting for a tick that is no longer running.
    reportUnsavedChanges(false);
    expect(sent).toEqual([true, false]);
  });

  it('sends the state as it stands when the binding finally appears', () => {
    const sent: boolean[] = [];
    reportUnsavedChanges(true);
    reportUnsavedChanges(false);
    bind((unsaved) => {
      sent.push(unsaved);
      return Promise.resolve();
    });
    vi.advanceTimersByTime(150);
    // Not two reports of history: the shell only ever needs the current answer.
    expect(sent).toEqual([false]);
  });

  it('gives up in a browser instead of polling forever', () => {
    reportUnsavedChanges(true);
    vi.advanceTimersByTime(6000);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('keeps editing working when the shell refuses the report', async () => {
    bind(() => Promise.reject(new Error('binding gone')));
    expect(() => reportUnsavedChanges(true)).not.toThrow();
    await Promise.resolve();
  });

  // The poller waits several seconds for a binding that never arrives in a
  // browser, so it can outlive the document that started it. A tick with no
  // `window` left to read threw a ReferenceError from a timer callback, where
  // nothing is waiting to catch it: an intermittent CI failure where the same
  // commit passed one run and failed the next. Both halves are covered, because
  // the tick has to stop as well as survive, and stopping used to read
  // `window.clearInterval` -- the one call that cannot work when the window is
  // what went away.
  it('survives the document going away while it is still polling', () => {
    const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'window');
    reportUnsavedChanges(true);
    expect(vi.getTimerCount()).toBe(1);
    try {
      // @ts-expect-error removing the global is the situation under test
      delete globalThis.window;
      expect(() => vi.advanceTimersByTime(150)).not.toThrow();
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      if (descriptor) Object.defineProperty(globalThis, 'window', descriptor);
    }
  });

  it('does not start polling when there is no window at all', () => {
    const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'window');
    try {
      // @ts-expect-error removing the global is the situation under test
      delete globalThis.window;
      expect(() => reportUnsavedChanges(true)).not.toThrow();
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      if (descriptor) Object.defineProperty(globalThis, 'window', descriptor);
    }
  });
});
