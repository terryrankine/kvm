import { useCallback } from "react";

import notifications from "@/notifications";
import { useHidStore, useRTCStore, useSettingsStore } from "@/hooks/stores";
import { useJsonRpc } from "@/hooks/useJsonRpc";
import { keys, modifiers } from "@/keyboardMappings";
import { chars } from "@/keyboardLayouts";

export interface MacroStep { keys: string[] | null; modifiers: string[] | null; delay: number }

/**
 * Converts a text string into a sequence of key-press steps using the given keyboard layout.
 * Mirrors the paste-text logic in usePasteHandler so macros can type freeform text.
 */
export function textToMacroSteps(text: string, layoutKey: string, delay: number): MacroStep[] {
  const normalizedLayout = (layoutKey || "").replace("-", "_");
  const safeLayout = normalizedLayout && chars[normalizedLayout] ? normalizedLayout : "en_US";
  const layoutChars = chars[safeLayout];
  const steps: MacroStep[] = [];

  for (const char of text) {
    const normalizedChar = char.normalize("NFC");
    const keyprops = layoutChars[normalizedChar];
    if (!keyprops) continue;

    const { key, shift, altRight, deadKey, accentKey } = keyprops;
    if (!key) continue;

    if (accentKey) {
      const accentMods: string[] = [];
      if (accentKey.shift) accentMods.push("ShiftLeft");
      if (accentKey.altRight) accentMods.push("AltRight");
      steps.push({
        keys: [String(accentKey.key)],
        modifiers: accentMods.length > 0 ? accentMods : null,
        delay,
      });
    }

    const charMods: string[] = [];
    if (shift) charMods.push("ShiftLeft");
    if (altRight) charMods.push("AltRight");
    steps.push({
      keys: [String(key)],
      modifiers: charMods.length > 0 ? charMods : null,
      delay,
    });

    if (deadKey) {
      steps.push({ keys: ["Space"], modifiers: null, delay });
    }
  }

  return steps;
}

export default function useKeyboard() {
  const [send] = useJsonRpc();

  const rpcDataChannel = useRTCStore(state => state.rpcDataChannel);
  const forceHttp = useSettingsStore(state => state.forceHttp);
  const keyboardLayout = useSettingsStore(state => state.keyboardLayout);
  const updateActiveKeysAndModifiers = useHidStore(
    state => state.updateActiveKeysAndModifiers,
  );
  const isReinitializingGadget = useHidStore(state => state.isReinitializingGadget);
  const usbState = useHidStore(state => state.usbState);

  const sendKeyboardEvent = useCallback(
    (keys: number[], modifiers: number[]) => {
      if (!forceHttp && rpcDataChannel?.readyState !== "open") return;
      // Don't send keyboard events while reinitializing gadget
      if (isReinitializingGadget) return;
      if (usbState !== "configured") return;
      const accModifier = modifiers.reduce((acc, val) => acc + val, 0);

      send("keyboardReport", { keys, modifier: accModifier }, resp => {
        if ("error" in resp) {
          const msg = (resp.error.data as string) || resp.error.message || "";
          if (msg.includes("cannot send after transport endpoint shutdown") && usbState === "configured") {
            notifications.error("Please check if the cable and connection are stable.", { duration: 5000 });
          }
        }
      });

      // We do this for the info bar to display the currently pressed keys for the user
      updateActiveKeysAndModifiers({ keys: keys, modifiers: modifiers });
    },
    [forceHttp, rpcDataChannel?.readyState, send, updateActiveKeysAndModifiers, isReinitializingGadget, usbState],
  );

  const resetKeyboardState = useCallback(() => {
    sendKeyboardEvent([], []);
  }, [sendKeyboardEvent]);

  const executeMacro = async (steps: { keys: string[] | null; modifiers: string[] | null; delay: number; text?: string }[]) => {
    // Expand any "Type Text" steps into individual key-press steps
    const expandedSteps: { keys: string[] | null; modifiers: string[] | null; delay: number }[] = [];
    for (const step of steps) {
      if (step.text !== undefined && step.text.length > 0) {
        expandedSteps.push(...textToMacroSteps(step.text, keyboardLayout, step.delay));
      } else {
        expandedSteps.push(step);
      }
    }

    for (const [index, step] of expandedSteps.entries()) {
      const keyValues = step.keys?.map(key => keys[key]).filter(Boolean) || [];
      const modifierValues = step.modifiers?.map(mod => modifiers[mod]).filter(Boolean) || [];

      // If the step has keys and/or modifiers, press them and hold for the delay
      if (keyValues.length > 0 || modifierValues.length > 0) {
        sendKeyboardEvent(keyValues, modifierValues);
        await new Promise(resolve => setTimeout(resolve, step.delay || 50));

        resetKeyboardState();
      } else {
        // This is a delay-only step, just wait for the delay amount
        await new Promise(resolve => setTimeout(resolve, step.delay || 50));
      }

      // Add a small pause between steps if not the last step
      if (index < expandedSteps.length - 1) {
        await new Promise(resolve => setTimeout(resolve, 10));
      }
    }
  };

  return { sendKeyboardEvent, resetKeyboardState, executeMacro };
}
