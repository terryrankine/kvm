import { useCallback, useEffect } from "react";

import { useSettingsStore, useUiStore } from "@/hooks/stores";
import { usePointerLock } from "./usePointerLock";

export const useFullscreen = (
  videoElm: React.RefObject<HTMLVideoElement>,
  pointerLock: ReturnType<typeof usePointerLock>,
  isFullscreen?: number
) => {
  const isFullscreenEnabled = document.fullscreenEnabled;
  const { setIsKeyboardLockActive } = useUiStore();
  const keyboardCaptureMode = useSettingsStore(state => state.keyboardCaptureMode);

  const requestKeyboardLock = useCallback(async () => {
    if (!("keyboard" in navigator)) return;

    try {
      // @ts-expect-error - keyboard lock API
      await navigator.keyboard.lock();
      console.debug("Keyboard lock acquired");
      setIsKeyboardLockActive(true);
    } catch (e) {
      console.debug("Keyboard lock not available:", e);
    }
  }, [setIsKeyboardLockActive]);

  const releaseKeyboardLock = useCallback(() => {
    if (!("keyboard" in navigator)) return;

    try {
      // @ts-expect-error - keyboard lock API
      navigator.keyboard.unlock();
      console.debug("Keyboard lock released");
    } catch {
      // ignore errors
    }
    setIsKeyboardLockActive(false);
  }, [setIsKeyboardLockActive]);

  const requestFullscreen = useCallback(async () => {
    if (!isFullscreenEnabled || !videoElm.current) return;

    await pointerLock.requestPointerLock();

    await videoElm.current.requestFullscreen({
      navigationUI: "show",
    });
    // keyboard.lock() is called in the fullscreenchange handler after fullscreen is confirmed
  }, [isFullscreenEnabled, pointerLock, videoElm]);

  useEffect(() => {
    if (isFullscreen) {
      requestFullscreen();
    }
  }, [isFullscreen]);

  // Handle fullscreen enter/exit: acquire or release keyboard lock accordingly
  useEffect(() => {
    const handleFullscreenChange = () => {
      if (document.fullscreenElement) {
        // Entering fullscreen: always acquire keyboard lock
        requestKeyboardLock();
      } else {
        // Exiting fullscreen: keep lock if capture mode is on, otherwise release
        if (keyboardCaptureMode) {
          requestKeyboardLock();
        } else {
          releaseKeyboardLock();
        }
      }
    };

    const abortController = new AbortController();
    document.addEventListener("fullscreenchange", handleFullscreenChange, {
      signal: abortController.signal,
    });
    return () => abortController.abort();
  }, [releaseKeyboardLock, requestKeyboardLock, keyboardCaptureMode]);

  // Sync keyboard lock with capture mode setting when not in fullscreen
  useEffect(
    function syncKeyboardCaptureMode() {
      if (keyboardCaptureMode) {
        requestKeyboardLock();
      } else if (!document.fullscreenElement) {
        releaseKeyboardLock();
      }
    },
    [keyboardCaptureMode, requestKeyboardLock, releaseKeyboardLock],
  );

  return {
    requestFullscreen,
    releaseKeyboardLock,
  };
};