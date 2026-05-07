import { useMemo } from "react";
import { useSettingsStore } from "@/hooks/stores";

export const useVideoEffects = () => {
  const videoSaturation = useSettingsStore(s => s.videoSaturation);
  const videoBrightness = useSettingsStore(s => s.videoBrightness);
  const videoContrast = useSettingsStore(s => s.videoContrast);
  const isCursorHidden = useSettingsStore(s => s.isCursorHidden);

  const videoStyle = useMemo(() => ({
    filter: `saturate(${videoSaturation}) brightness(${videoBrightness}) contrast(${videoContrast})`,
    // Promote to its own compositor layer so filter changes don't trigger a
    // full-page repaint on every frame adjustment.
    willChange: "filter" as const,
    transform: "translateZ(0)",
  }), [videoSaturation, videoBrightness, videoContrast]);

  return {
    // Expose only what callers use to avoid unnecessary re-renders.
    settings: { isCursorHidden },
    videoStyle,
    videoSaturation,
    videoBrightness,
    videoContrast,
  };
};
