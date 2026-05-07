import { useCallback } from "react";

import  useKeyboard  from "@/hooks/useKeyboard";
import { useHidStore, useSettingsStore } from "@/hooks/stores";
import { keys, modifiers } from "@/keyboardMappings";

export const useKeyboardEvents = (
  pasteCaptureRef?: React.RefObject<HTMLTextAreaElement>,
  isReinitializingGadget?: boolean
) => {
  const { sendKeyboardEvent, resetKeyboardState } = useKeyboard();
  const { setIsNumLockActive, setIsCapsLockActive, setIsScrollLockActive } = useHidStore();

  const keyboardLedStateSyncAvailable = useHidStore(state => state.keyboardLedStateSyncAvailable);
  const keyboardLedSync = useSettingsStore(state => state.keyboardLedSync);
  const isKeyboardLedManagedByHost = keyboardLedSync !== "browser" && keyboardLedStateSyncAvailable;
  const overrideCtrlV = useSettingsStore(state => state.overrideCtrlV);

  const handleModifierKeys = useCallback((e: KeyboardEvent, activeModifiers: number[]) => {
    const { shiftKey, ctrlKey, altKey, metaKey } = e;
    const filteredModifiers = activeModifiers.filter(Boolean);

    return filteredModifiers
      .filter(modifier => shiftKey || (modifier !== modifiers["ShiftLeft"] && modifier !== modifiers["ShiftRight"]))
      .filter(modifier => ctrlKey || (modifier !== modifiers["ControlLeft"] && modifier !== modifiers["ControlRight"]))
      .filter(modifier => altKey || modifier !== modifiers["AltLeft"])
      .filter(modifier => metaKey || (modifier !== modifiers["MetaLeft"] && modifier !== modifiers["MetaRight"]));
  }, []);

  // Firefox resistFingerprinting (RFP) suppresses standalone modifier key events.
  // When Shift+A is pressed, only the KeyA event fires with shiftKey=true; no
  // ShiftLeft keydown precedes it. Reconcile by adding any modifier indicated by
  // the event's boolean flags that is not already present in the modifier list.
  // We use the Left-side modifier as the synthetic fallback to match upstream behaviour.
  const reconcileModifiers = useCallback((e: KeyboardEvent, currentModifiers: number[]): number[] => {
    const synthetic: number[] = [];
    const mapping: [boolean, string][] = [
      [e.shiftKey, "ShiftLeft"],
      [e.ctrlKey, "ControlLeft"],
      [e.altKey, "AltLeft"],
      [e.metaKey, "MetaLeft"],
    ];
    for (const [active, name] of mapping) {
      const hidCode = modifiers[name];
      // If the flag says the modifier is active but it's not yet in the list,
      // inject it — unless this event IS for that modifier key (normal path handles it).
      if (active && !currentModifiers.includes(hidCode) && modifiers[e.code] !== hidCode) {
        synthetic.push(hidCode);
      }
    }
    return synthetic.length > 0 ? [...currentModifiers, ...synthetic] : currentModifiers;
  }, []);

  const keyDownHandler = useCallback(async (e: KeyboardEvent) => {
    if (overrideCtrlV && (e.code === "KeyV" || e.key.toLowerCase() === "v") && (e.ctrlKey || e.metaKey)) {
        console.log("Override Ctrl V");
        if (isReinitializingGadget) return;
        if (pasteCaptureRef && pasteCaptureRef.current) {
          pasteCaptureRef.current.value = "";
          pasteCaptureRef.current.focus();
        }
        return;
      }
    if (isReinitializingGadget) return;

    e.preventDefault();
    const prev = useHidStore.getState();
    let code = e.code;
    const key = e.key;

    if (!isKeyboardLedManagedByHost) {
      setIsNumLockActive(e.getModifierState("NumLock"));
      setIsCapsLockActive(e.getModifierState("CapsLock"));
      setIsScrollLockActive(e.getModifierState("ScrollLock"));
    }

    if (code == "IntlBackslash" && ["`", "~"].includes(key)) {
      code = "Backquote";
    } else if (code == "Backquote" && ["§", "±"].includes(key)) {
      code = "IntlBackslash";
    }

    const newKeys = [...prev.activeKeys, keys[code]].filter(Boolean);
    const baseModifiers = handleModifierKeys(e, [...prev.activeModifiers, modifiers[code]]);
    const newModifiers = reconcileModifiers(e, baseModifiers);

    if (e.metaKey) {
      setTimeout(() => {
        const prev = useHidStore.getState();
        sendKeyboardEvent([], newModifiers || prev.activeModifiers);
      }, 10);
    }

    sendKeyboardEvent([...new Set(newKeys)], [...new Set(newModifiers)]);
  }, [handleModifierKeys, reconcileModifiers, sendKeyboardEvent, isKeyboardLedManagedByHost, setIsNumLockActive, setIsCapsLockActive, setIsScrollLockActive, overrideCtrlV, pasteCaptureRef, isReinitializingGadget]);

  const keyUpHandler = useCallback((e: KeyboardEvent) => {
    if (isReinitializingGadget) return;
    e.preventDefault();
    const prev = useHidStore.getState();

    if (!isKeyboardLedManagedByHost) {
      setIsNumLockActive(e.getModifierState("NumLock"));
      setIsCapsLockActive(e.getModifierState("CapsLock"));
      setIsScrollLockActive(e.getModifierState("ScrollLock"));
    }

    const newKeys = prev.activeKeys.filter(k => k !== keys[e.code]).filter(Boolean);
    const newModifiers = handleModifierKeys(
      e,
      prev.activeModifiers.filter(k => k !== modifiers[e.code]),
    );

    sendKeyboardEvent([...new Set(newKeys)], [...new Set(newModifiers)]);
  }, [handleModifierKeys, sendKeyboardEvent, isKeyboardLedManagedByHost, setIsNumLockActive, setIsCapsLockActive, setIsScrollLockActive]);

  const setupKeyboardEvents = useCallback(() => {
    const abortController = new AbortController();
    const signal = abortController.signal;

    document.addEventListener("keydown", keyDownHandler, { signal });
    document.addEventListener("keyup", keyUpHandler, { signal });
    window.addEventListener("blur", resetKeyboardState, { signal });
    document.addEventListener("visibilitychange", resetKeyboardState, { signal });

    return () => abortController.abort();
  }, [keyDownHandler, keyUpHandler, resetKeyboardState]);

  return {
    setupKeyboardEvents,
  };
};