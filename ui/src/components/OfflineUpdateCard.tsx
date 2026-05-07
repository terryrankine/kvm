import { useCallback, useRef, useState } from "react";
import { Button as AntdButton } from "antd";
import { useReactAt } from "i18n-auto-extractor/react";
import { CheckCircleIcon } from "@heroicons/react/20/solid";

import { SettingsItem } from "@components/Settings/SettingsView";
import { DEVICE_API } from "@/ui.config";

interface UploadResult {
  verified: boolean;
  hashOK: boolean;
  error?: string;
}

type UploadState = "idle" | "uploading" | "verifying" | "verified" | "applying" | "error";

interface ComponentUploadState {
  state: UploadState;
  progress: number;
  result: UploadResult | null;
  error: string | null;
}

const initialState: ComponentUploadState = {
  state: "idle",
  progress: 0,
  result: null,
  error: null,
};

function ComponentUpload({ component, label }: { component: string; label: string }) {
  const { $at } = useReactAt();
  const [upload, setUpload] = useState<ComponentUploadState>(initialState);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileSelect = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file) return;

      if (!file.name.endsWith(".tar.gz") && !file.name.endsWith(".tgz")) {
        setUpload({ ...initialState, state: "error", error: $at("Please select a .tar.gz archive") });
        return;
      }

      const formData = new FormData();
      formData.append("component", component);
      formData.append("file", file);

      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${DEVICE_API}/ota/upload`, true);

      setUpload({ ...initialState, state: "uploading" });

      xhr.upload.onprogress = event => {
        if (event.lengthComputable) {
          const pct = Math.round((event.loaded / event.total) * 100);
          setUpload(prev => ({ ...prev, progress: pct }));
          if (pct >= 100) {
            setUpload(prev => ({ ...prev, state: "verifying" }));
          }
        }
      };

      xhr.onload = () => {
        try {
          const result: UploadResult = JSON.parse(xhr.responseText);
          if (xhr.status === 200 && result.verified) {
            setUpload({ state: "verified", progress: 100, result, error: null });
          } else {
            setUpload({
              state: "error",
              progress: 0,
              result: null,
              error: result.error || `HTTP ${xhr.status}`,
            });
          }
        } catch {
          setUpload({
            state: "error",
            progress: 0,
            result: null,
            error: xhr.statusText || $at("Unknown error"),
          });
        }
      };

      xhr.onerror = () => {
        setUpload({ state: "error", progress: 0, result: null, error: $at("Network error") });
      };

      xhr.send(formData);

      // Reset so re-selecting the same file triggers onChange again.
      if (fileInputRef.current) fileInputRef.current.value = "";
    },
    [component, $at],
  );

  const handleApply = useCallback(() => {
    setUpload(prev => ({ ...prev, state: "applying" }));
    fetch(`${DEVICE_API}/ota/apply`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ component }),
    }).catch(() => {
      // Expected: device reboots and drops the connection.
    });
  }, [component]);

  const reset = useCallback(() => setUpload(initialState), []);

  return (
    <div className="space-y-3">
      <p className="text-sm font-semibold text-black dark:text-white">{label}</p>

      {upload.state === "idle" && (
        <div>
          <AntdButton type="primary" onClick={() => fileInputRef.current?.click()}>
            {$at("Select File")}
          </AntdButton>
          <input
            ref={fileInputRef}
            type="file"
            accept=".tar.gz,.tgz"
            onChange={handleFileSelect}
            className="hidden"
          />
        </div>
      )}

      {(upload.state === "uploading" || upload.state === "verifying") && (
        <div className="space-y-1">
          <div className="flex items-center justify-between text-sm text-slate-600 dark:text-slate-400">
            <span>
              {upload.state === "uploading" ? $at("Uploading…") : $at("Verifying…")}
            </span>
            <span className="font-mono text-[13px]">{upload.progress}%</span>
          </div>
          <div className="h-2 w-full overflow-hidden rounded-full bg-slate-200 dark:bg-slate-700">
            <div
              className="h-2 rounded-full bg-blue-600 transition-all duration-300"
              style={{ width: `${upload.progress}%` }}
            />
          </div>
        </div>
      )}

      {upload.state === "verified" && upload.result && (
        <div className="space-y-2">
          <div className="flex items-center gap-x-2 text-sm text-green-700 dark:text-green-400">
            <CheckCircleIcon className="h-4 w-4" />
            <span>{$at("SHA-256 hash verified")}</span>
          </div>
          <div className="flex gap-x-2">
            <AntdButton type="primary" onClick={handleApply}>
              {$at("Apply Update")}
            </AntdButton>
            <AntdButton onClick={reset}>{$at("Cancel")}</AntdButton>
          </div>
        </div>
      )}

      {upload.state === "applying" && (
        <p className="text-sm text-slate-600 dark:text-slate-400">
          {$at("Applying update… Device will reboot.")}
        </p>
      )}

      {upload.state === "error" && upload.error && (
        <div className="space-y-2">
          <p className="text-sm text-red-600 dark:text-red-400">
            {$at("Upload failed")}: {upload.error}
          </p>
          <AntdButton onClick={reset}>{$at("Retry")}</AntdButton>
        </div>
      )}
    </div>
  );
}

export default function OfflineUpdateCard() {
  const { $at } = useReactAt();
  return (
    <div className="space-y-4">
      <SettingsItem
        title={$at("Offline Update")}
        description={$at("Upload a signed .tar.gz firmware archive to update without internet access.")}
        noCol
      />
      <div className="space-y-4 rounded-md border border-slate-200 p-4 dark:border-slate-700">
        <ComponentUpload component="app" label={$at("App Update")} />
        <hr className="border-slate-200 dark:border-slate-700" />
        <ComponentUpload component="system" label={$at("System Update")} />
      </div>
    </div>
  );
}
