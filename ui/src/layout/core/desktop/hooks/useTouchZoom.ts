import { useEffect, useRef, useState } from "react";

export const useTouchZoom = (
  containerRef: React.RefObject<HTMLDivElement>
) => {
  // State is only updated at the END of a gesture (pointerup/reset) so React
  // doesn't re-render on every pointermove. During active pinch/pan the
  // transform is applied directly to the DOM for smooth 60fps updates.
  const [mobileScale, setMobileScale] = useState(1);
  const [mobileTx, setMobileTx] = useState(0);
  const [mobileTy, setMobileTy] = useState(0);

  // Refs for real-time values — readable by useMouseEvents without triggering renders.
  const mobileScaleRef = useRef(1);
  const mobileTxRef = useRef(0);
  const mobileTyRef = useRef(0);

  const activeTouchPointers = useRef<Map<number, { x: number; y: number }>>(new Map());
  const initialPinchDistance = useRef<number | null>(null);
  const initialPinchScale = useRef<number>(1);
  const lastPanPoint = useRef<{ x: number; y: number } | null>(null);
  const lastTapAt = useRef<number>(0);

  const applyTransform = (scale: number, tx: number, ty: number) => {
    const el = containerRef.current;
    if (el) {
      el.style.transform = `translate(${tx}px, ${ty}px) scale(${scale})`;
    }
    mobileScaleRef.current = scale;
    mobileTxRef.current = tx;
    mobileTyRef.current = ty;
  };

  const clampAndApply = (scale: number, tx: number, ty: number) => {
    const el = containerRef.current;
    const cw = el?.clientWidth ?? 0;
    const ch = el?.clientHeight ?? 0;
    const maxX = cw ? (cw * (scale - 1)) / 2 : 0;
    const maxY = ch ? (ch * (scale - 1)) / 2 : 0;
    const clampedTx = Math.max(-maxX, Math.min(maxX, tx));
    const clampedTy = Math.max(-maxY, Math.min(maxY, ty));
    applyTransform(scale, clampedTx, clampedTy);
    return { scale, tx: clampedTx, ty: clampedTy };
  };

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const abortController = new AbortController();
    const signal = abortController.signal;

    const onPointerDown = (e: PointerEvent) => {
      if (e.pointerType !== "touch") return;
      activeTouchPointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY });
      if (activeTouchPointers.current.size === 1) {
        const now = Date.now();
        let isInVideo = false;
        const video = el.querySelector("video") as HTMLVideoElement | null;
        if (video) {
          const vRect = video.getBoundingClientRect();
          if (
            e.clientX >= vRect.left &&
            e.clientX <= vRect.right &&
            e.clientY >= vRect.top &&
            e.clientY <= vRect.bottom
          ) {
            isInVideo = true;
          }
        }
        if (!isInVideo) {
          if (now - lastTapAt.current < 300) {
            const r = clampAndApply(1, 0, 0);
            setMobileScale(r.scale);
            setMobileTx(r.tx);
            setMobileTy(r.ty);
          }
        }
        lastTapAt.current = now;
        lastPanPoint.current = { x: e.clientX, y: e.clientY };
      } else if (activeTouchPointers.current.size === 2) {
        const pts = Array.from(activeTouchPointers.current.values());
        const d = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
        initialPinchDistance.current = d;
        initialPinchScale.current = mobileScaleRef.current;
      }
      e.preventDefault();
      e.stopPropagation();
    };

    const onPointerMove = (e: PointerEvent) => {
      if (e.pointerType !== "touch") return;
      const prev = activeTouchPointers.current.get(e.pointerId);
      activeTouchPointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY });
      const pts = Array.from(activeTouchPointers.current.values());
      if (pts.length === 2 && initialPinchDistance.current) {
        const d = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
        const factor = d / initialPinchDistance.current;
        const next = Math.max(1, Math.min(4, initialPinchScale.current * factor));
        // Apply directly to DOM — no setState on every move event.
        applyTransform(next, mobileTxRef.current, mobileTyRef.current);
      } else if (pts.length === 1 && lastPanPoint.current && prev) {
        const dx = e.clientX - lastPanPoint.current.x;
        const dy = e.clientY - lastPanPoint.current.y;
        lastPanPoint.current = { x: e.clientX, y: e.clientY };
        applyTransform(
          mobileScaleRef.current,
          mobileTxRef.current + dx,
          mobileTyRef.current + dy,
        );
      }
      e.preventDefault();
      e.stopPropagation();
    };

    const onPointerUp = (e: PointerEvent) => {
      if (e.pointerType !== "touch") return;
      activeTouchPointers.current.delete(e.pointerId);
      if (activeTouchPointers.current.size < 2) {
        initialPinchDistance.current = null;
      }
      if (activeTouchPointers.current.size === 0) {
        lastPanPoint.current = null;
        // Clamp and sync to React state once the gesture ends.
        const r = clampAndApply(
          mobileScaleRef.current,
          mobileTxRef.current,
          mobileTyRef.current,
        );
        setMobileScale(r.scale);
        setMobileTx(r.tx);
        setMobileTy(r.ty);
      }
      e.preventDefault();
      e.stopPropagation();
    };

    el.addEventListener("pointerdown", onPointerDown, { signal });
    el.addEventListener("pointermove", onPointerMove, { signal });
    el.addEventListener("pointerup", onPointerUp, { signal });
    el.addEventListener("pointercancel", onPointerUp, { signal });

    return () => abortController.abort();
  }, [containerRef]);

  return {
    mobileScale,
    mobileTx,
    mobileTy,
    // Expose refs for hooks (useMouseEvents) that need current values without
    // subscribing to React state.
    mobileScaleRef,
    activeTouchPointers,
    lastPanPoint,
  };
};
