// Draggable, keyboard-accessible vertical splitter between two workspace
// panes. Pointer Events with capture handle mouse/pen/touch uniformly; drags
// report the total delta from the drag origin so pointer math cannot drift.
//
// Callback props are read through refs ("latest ref" pattern): parent
// re-renders during a drag (async sidebar/status updates) must not recreate
// the cleanup effect, which would otherwise cancel the user's drag mid-move.
import { useCallback, useEffect, useRef } from 'react';

const KEY_STEP = 16;
const KEY_STEP_LARGE = 64;

export interface PaneSplitterProps {
  /** Accessible label, e.g. "Resize sidebar". */
  label: string;
  /** Current width of the pane this splitter controls (aria-valuenow). */
  value: number;
  min: number;
  max: number;
  /** Called during drags with the total pixel delta since the drag started. */
  onDrag: (totalDelta: number) => void;
  /** Marks the start of a drag so the owner can snapshot widths. */
  onDragStart: () => void;
  onDragEnd: () => void;
  /** Keyboard resize by a signed pixel step. */
  onStep: (delta: number) => void;
  /** Double-click restores default sizes. */
  onReset: () => void;
}

export function PaneSplitter({ label, value, min, max, onDrag, onDragStart, onDragEnd, onStep, onReset }: PaneSplitterProps) {
  const dragRef = useRef<{ pointerId: number; originX: number } | null>(null);
  const frameRef = useRef(0);
  const pendingDeltaRef = useRef(0);
  const elementRef = useRef<HTMLDivElement>(null);
  const windowListenersRef = useRef<(() => void) | null>(null);

  const callbacksRef = useRef({ onDrag, onDragStart, onDragEnd, onStep, onReset });
  useEffect(() => {
    callbacksRef.current = { onDrag, onDragStart, onDragEnd, onStep, onReset };
  });

  const endDrag = useCallback(() => {
    if (dragRef.current === null) return;
    const element = elementRef.current;
    if (element && element.hasPointerCapture(dragRef.current.pointerId)) {
      element.releasePointerCapture(dragRef.current.pointerId);
    }
    windowListenersRef.current?.();
    windowListenersRef.current = null;
    dragRef.current = null;
    cancelAnimationFrame(frameRef.current);
    document.body.classList.remove('splitter-dragging');
    callbacksRef.current.onDragEnd();
  }, []);

  // End drags on window blur (e.g. the Wails window loses focus) and clean up
  // on unmount so no listeners or drag state leak. Mount-only: endDrag is
  // stable, so re-renders never cancel an in-progress drag.
  useEffect(() => {
    const onBlur = () => endDrag();
    window.addEventListener('blur', onBlur);
    return () => {
      window.removeEventListener('blur', onBlur);
      endDrag();
    };
  }, [endDrag]);

  const trackMove = useCallback((pointerId: number, clientX: number) => {
    const drag = dragRef.current;
    if (!drag || pointerId !== drag.pointerId) return;
    // Coalesce moves into one state update per animation frame.
    pendingDeltaRef.current = clientX - drag.originX;
    cancelAnimationFrame(frameRef.current);
    frameRef.current = requestAnimationFrame(() => {
      if (dragRef.current) callbacksRef.current.onDrag(pendingDeltaRef.current);
    });
  }, []);

  const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0 && event.pointerType === 'mouse') return;
    event.preventDefault();
    dragRef.current = { pointerId: event.pointerId, originX: event.clientX };
    try {
      elementRef.current?.setPointerCapture(event.pointerId);
    } catch {
      // Pointer capture can be unavailable (synthetic pointers, some
      // embedded webviews); fall back to window-level tracking so fast
      // drags don't escape the 6px splitter.
      const drag = dragRef.current;
      const onWindowMove = (moveEvent: PointerEvent) => trackMove(moveEvent.pointerId, moveEvent.clientX);
      const onWindowUp = (upEvent: PointerEvent) => {
        if (upEvent.pointerId === drag.pointerId) endDrag();
      };
      window.addEventListener('pointermove', onWindowMove);
      window.addEventListener('pointerup', onWindowUp);
      window.addEventListener('pointercancel', onWindowUp);
      windowListenersRef.current = () => {
        window.removeEventListener('pointermove', onWindowMove);
        window.removeEventListener('pointerup', onWindowUp);
        window.removeEventListener('pointercancel', onWindowUp);
      };
    }
    document.body.classList.add('splitter-dragging'); // suppresses text selection
    callbacksRef.current.onDragStart();
  };

  const onPointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    trackMove(event.pointerId, event.clientX);
  };

  const onKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    const step = event.shiftKey ? KEY_STEP_LARGE : KEY_STEP;
    if (event.key === 'ArrowLeft') {
      event.preventDefault();
      callbacksRef.current.onStep(-step);
    } else if (event.key === 'ArrowRight') {
      event.preventDefault();
      callbacksRef.current.onStep(step);
    } else if (event.key === 'Home') {
      event.preventDefault();
      callbacksRef.current.onStep(min - value);
    } else if (event.key === 'End') {
      event.preventDefault();
      callbacksRef.current.onStep(max - value);
    }
  };

  return (
    <div
      ref={elementRef}
      className="pane-splitter"
      role="separator"
      aria-orientation="vertical"
      aria-label={label}
      aria-valuenow={Math.round(value)}
      aria-valuemin={Math.round(min)}
      aria-valuemax={Math.round(max)}
      tabIndex={0}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={endDrag}
      onPointerCancel={endDrag}
      onLostPointerCapture={endDrag}
      onKeyDown={onKeyDown}
      onDoubleClick={() => callbacksRef.current.onReset()}
      data-testid="pane-splitter"
    />
  );
}
