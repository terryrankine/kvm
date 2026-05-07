import { useCallback, useEffect, useMemo, useState, useRef } from "react";
import { isMobile } from "react-device-detect";

import { useJsonRpc } from "@/hooks/useJsonRpc";
import { useMouseStore, useSettingsStore, useVideoStore, useHidStore } from "@/hooks/stores";

import { usePointerLock } from "./usePointerLock";

export const useMouseEvents = (
  videoElm: React.RefObject<HTMLVideoElement>,
  pointerLock: ReturnType<typeof usePointerLock>,
  touchZoom?: {
    mobileScale: number;
    mobileScaleRef?: React.MutableRefObject<number>;
    mobileTx: number;
    mobileTy: number;
    activeTouchPointers: React.MutableRefObject<Map<number, { x: number; y: number }>>;
    lastPanPoint: React.MutableRefObject<{ x: number; y: number } | null>;
  },
  disableTouchClick?: boolean,
  externalButtons = 0,
  containerRef?: React.RefObject<HTMLDivElement>,
) => {
  const [send] = useJsonRpc();
  const [blockWheelEvent, setBlockWheelEvent] = useState(false);
  const mouseMode = useSettingsStore(s => s.mouseMode);
  const scrollThrottling = useSettingsStore(s => s.scrollThrottling);
  const setMousePosition = useMouseStore(s => s.setMousePosition);
  const setMouseMove = useMouseStore(s => s.setMouseMove);
  const videoWidth = useVideoStore(s => s.width);
  const videoHeight = useVideoStore(s => s.height);
  const displayedWidth = useVideoStore(s => s.clientWidth);
  const displayedHeight = useVideoStore(s => s.clientHeight);
  const streamContentX1 = useVideoStore(s => s.streamContentX1);
  const streamContentX2 = useVideoStore(s => s.streamContentX2);
  const streamContentY1 = useVideoStore(s => s.streamContentY1);
  const streamContentY2 = useVideoStore(s => s.streamContentY2);
  const isReinitializingGadget = useHidStore(state => state.isReinitializingGadget);

  const calcDelta = (pos: number) => (Math.abs(pos) < 10 ? pos * 2 : pos);

  // Coordinate mapper: computed once per resize/stream-change, not per mouse event.
  // Combines CSS pillarbox/letterbox correction and hardware stream bar correction.
  const coordMapper = useMemo(() => {
    if (!displayedWidth || !displayedHeight || !videoWidth || !videoHeight) return null;

    // CSS correction: object-fit contain adds bars when element AR ≠ stream AR
    const elemAR = displayedWidth / displayedHeight;
    const streamAR = videoWidth / videoHeight;
    let ew = displayedWidth, eh = displayedHeight, ox = 0, oy = 0;
    if (elemAR > streamAR) {
      ew = displayedHeight * streamAR;
      ox = (displayedWidth - ew) / 2;
    } else if (elemAR < streamAR) {
      eh = displayedWidth / streamAR;
      oy = (displayedHeight - eh) / 2;
    }

    // Stream bar correction: hardware encodes limited-range YUV black (RGB 16,16,16)
    // when HDMI input resolution ≠ encode resolution. X and Y are independent —
    // scale-to-fit always bars one axis only.
    const minBar = 5;
    const hasX = streamContentX1 > minBar && streamContentX2 > streamContentX1;
    const hasY = streamContentY1 > minBar && streamContentY2 > streamContentY1;

    return {
      cssOffsetX: ox,
      cssOffsetY: oy,
      effectiveWidth: ew,
      effectiveHeight: eh,
      hasX,
      hasY,
      // Pre-compute stream bar denominators to avoid division per event
      xContentOffset: hasX ? streamContentX1 : 0,
      xContentWidth: hasX ? streamContentX2 - streamContentX1 + 1 : videoWidth,
      yContentOffset: hasY ? streamContentY1 : 0,
      yContentHeight: hasY ? streamContentY2 - streamContentY1 + 1 : videoHeight,
      videoWidth,
      videoHeight,
    };
  }, [displayedWidth, displayedHeight, videoWidth, videoHeight,
      streamContentX1, streamContentX2, streamContentY1, streamContentY2]);

  const sendRelMouseMovement = useCallback(
    (x: number, y: number, buttons: number) => {
      if (mouseMode !== "relative") return;
      if (isReinitializingGadget) return;
      send("relMouseReport", { dx: calcDelta(x), dy: calcDelta(y), buttons });
      setMouseMove({ x, y, buttons });
    },
    [send, setMouseMove, mouseMode, isReinitializingGadget],
  );

  const sendAbsMouseMovement = useCallback(
    (x: number, y: number, buttons: number) => {
      if (mouseMode !== "absolute") return;
      if (isReinitializingGadget) return;
      send("absMouseReport", { x, y, buttons });
      setMousePosition(x, y);
    },
    [send, setMousePosition, mouseMode, isReinitializingGadget],
  );

  const relMouseMoveHandler = useCallback(
    (e: MouseEvent) => {
      const pt = (e as unknown as PointerEvent).pointerType as unknown as string;
      if (pt === "touch") {
        if (touchZoom) {
            const touchCount = touchZoom.activeTouchPointers.current.size;
            if (touchCount >= 2) return;
            const scale = touchZoom.mobileScaleRef?.current ?? touchZoom.mobileScale;
            if (scale > 1 && touchZoom.lastPanPoint.current) return;
        }
      }
      if (isMobile) e.preventDefault();
      if (mouseMode !== "relative") return;
      if (!pointerLock.isPointerLockActive && pointerLock.isPointerLockPossible) return;

      sendRelMouseMovement(e.movementX, e.movementY, e.buttons);
    },
    [pointerLock.isPointerLockActive, pointerLock.isPointerLockPossible, sendRelMouseMovement, mouseMode, touchZoom],
  );

  const absMouseMoveHandler = useCallback(
    (e: MouseEvent) => {
      if (!coordMapper) return;
      if (mouseMode !== "absolute") return;

      const pt = (e as unknown as PointerEvent).pointerType as unknown as string;
      if (pt === "touch") {
        if (touchZoom) {
          const touchCount = touchZoom.activeTouchPointers.current.size;
          const eventType = (e as unknown as PointerEvent).type;
          if (touchCount >= 2 && eventType !== "pointerup") return;
        }
      }

      if (isMobile) e.preventDefault();

      const { cssOffsetX, cssOffsetY, effectiveWidth, effectiveHeight,
              hasX, hasY, xContentOffset, xContentWidth, yContentOffset, yContentHeight,
              videoWidth: vw, videoHeight: vh } = coordMapper;

      // Clamp to CSS content area, compute stream fractions
      const cx = Math.min(Math.max(cssOffsetX, e.offsetX), cssOffsetX + effectiveWidth);
      const cy = Math.min(Math.max(cssOffsetY, e.offsetY), cssOffsetY + effectiveHeight);
      const fx = (cx - cssOffsetX) / effectiveWidth;
      const fy = (cy - cssOffsetY) / effectiveHeight;

      // Map stream fraction → HID 0–32767, applying stream bar correction if detected
      const x = Math.round((hasX
        ? Math.min(Math.max(0, (fx * vw - xContentOffset) / xContentWidth), 1)
        : fx) * 32767);
      const y = Math.round((hasY
        ? Math.min(Math.max(0, (fy * vh - yContentOffset) / yContentHeight), 1)
        : fy) * 32767);

      let buttons = e.buttons;
      if (pt === "touch") {
        const touchCount = touchZoom ? touchZoom.activeTouchPointers.current.size : 1;
        const eventType = (e as unknown as PointerEvent).type;
        if (eventType === "pointerup") {
          buttons = (touchCount >= 2 || disableTouchClick) ? 0 : 1;
        } else {
          buttons = 0;
        }
      }

      buttons |= externalButtons;
      sendAbsMouseMovement(x, y, buttons);

      if (pt === "touch" && buttons !== externalButtons && (e as unknown as PointerEvent).type === "pointerup") {
        sendAbsMouseMovement(x, y, externalButtons);
      }
    },
    [coordMapper, mouseMode, sendAbsMouseMovement, touchZoom, disableTouchClick, externalButtons],
  );

  const mouseWheelHandler = useCallback(
    (e: WheelEvent) => {
      if (isReinitializingGadget) return;
      if (scrollThrottling && blockWheelEvent) return;

      const isAccel = Math.abs(e.deltaY) >= 100;
      const scrollValue = isAccel ? e.deltaY / 100 : Math.sign(e.deltaY);
      const invertedScrollValue = -Math.max(-127, Math.min(127, scrollValue));

      send("wheelReport", { wheelY: invertedScrollValue });

      if (scrollThrottling && !blockWheelEvent) {
        setBlockWheelEvent(true);
        setTimeout(() => setBlockWheelEvent(false), scrollThrottling);
      }
    },
    [send, blockWheelEvent, scrollThrottling, isReinitializingGadget],
  );

  const resetMousePosition = useCallback(() => {
    sendAbsMouseMovement(0, 0, 0);
  }, [sendAbsMouseMovement]);

  const isRelativeMouseMode = (mouseMode === "relative");
  const mouseMoveHandler = isRelativeMouseMode ? relMouseMoveHandler : absMouseMoveHandler;
  const handlerRef = useRef(mouseMoveHandler);

  useEffect(() => {
    handlerRef.current = mouseMoveHandler;
  }, [mouseMoveHandler]);

  const setupMouseEvents = useCallback(() => {
    const videoElmRefValue = videoElm.current;
    if (!videoElmRefValue) return;

    const abortController = new AbortController();
    const signal = abortController.signal;

    const eventHandler = (e: Event) => {
      if (handlerRef.current) handlerRef.current(e as any);
    };

    videoElmRefValue.addEventListener("mousemove", eventHandler, { signal });
    videoElmRefValue.addEventListener("pointerdown", eventHandler, { signal });
    videoElmRefValue.addEventListener("pointerup", eventHandler, { signal });
    videoElmRefValue.addEventListener("wheel", mouseWheelHandler, { signal, passive: true });

    if (isRelativeMouseMode) {
      // Use containerRef so pointer lock can be requested even when HDMI has no
      // signal and the video element is hidden/empty (PR #822).
      const pointerLockTarget = containerRef?.current ?? videoElmRefValue;
      pointerLockTarget.addEventListener("click",
        () => {
          if (pointerLock.isPointerLockPossible && !pointerLock.isPointerLockActive && !document.pointerLockElement) {
            pointerLock.requestPointerLock();
          }
        },
        { signal },
      );
    } else {
      window.addEventListener("blur", resetMousePosition, { signal });
      document.addEventListener("visibilitychange", resetMousePosition, { signal });
    }

    videoElmRefValue.addEventListener("contextmenu", (e: MouseEvent) => e.preventDefault(), { signal });

    return () => abortController.abort();
  }, [
    videoElm,
    mouseMode,
    isRelativeMouseMode,
    mouseWheelHandler,
    pointerLock,
    resetMousePosition,
    containerRef,
  ]);

  return { setupMouseEvents };
};
