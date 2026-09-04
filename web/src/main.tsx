import React from 'react';
import ReactDOM from 'react-dom/client';
import { App } from './App';
import './styles.css';

/**
 * Records an uncaught frontend failure where somebody can read it.
 *
 * A React render that throws unmounts the whole tree, and what a person sees is
 * an empty window: no message, no clue, and nothing in any log, because the
 * service is perfectly healthy and never hears about it. That happened while
 * the About dialog was being written, and three plausible causes were guessed
 * at before anyone thought to ask the application what had actually gone wrong.
 *
 * In the desktop shell this goes into the same transcript the About dialog
 * offers to copy, so a bug report carries the error rather than a description
 * of a blank screen. In a browser it falls back to the console, which is at
 * least somewhere a developer can look.
 */
function reportFailure(kind: string, detail: string): void {
  const bound = (window as Window & {
    go?: { main?: { NativeUIBridge?: { LogAction?: (category: string, detail: string) => void } } };
  }).go?.main?.NativeUIBridge;
  try {
    bound?.LogAction?.('error', `${kind}: ${detail}`);
  } catch {
    // Reporting a failure must never be the thing that fails.
  }
  console.error(`notrios ${kind}:`, detail);
}

window.addEventListener('error', (event) => {
  reportFailure('uncaught', `${event.message} at ${event.filename}:${event.lineno}:${event.colno}`);
});
window.addEventListener('unhandledrejection', (event) => {
  reportFailure('unhandled rejection', String(event.reason instanceof Error ? event.reason.stack ?? event.reason.message : event.reason));
});

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
