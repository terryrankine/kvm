import { useState } from "react";
import { LuPlus, LuX } from "react-icons/lu";
import { useReactAt } from "i18n-auto-extractor/react";

import { NetworkSettings } from "@/hooks/stores";
import { Button } from "@components/Button";
import { GridCard } from "@components/Card";
import FieldLabel from "@components/FieldLabel";
import { FieldError, InputFieldWithLabel } from "@components/InputField";

// Display names for time source ordering options
const TIME_SOURCE_LABELS: Record<string, string> = {
  ntp: "Built-in NTP",
  http: "Built-in HTTP",
  ntp_dhcp: "DHCP-provided NTP",
  ntp_user_provided: "Custom NTP servers",
  http_user_provided: "Custom HTTP URLs",
};

const ALL_SOURCES = Object.keys(TIME_SOURCE_LABELS);

const ensureArray = <T,>(arr: T[] | null | undefined): T[] =>
  Array.isArray(arr) ? arr : [];

interface CustomTimeConfigurationCardProps {
  settings: NetworkSettings;
  onChange: (updated: Partial<NetworkSettings>) => void;
}

export default function CustomTimeConfigurationCard({
  settings,
  onChange,
}: CustomTimeConfigurationCardProps) {
  const { $at } = useReactAt();
  const [errors, setErrors] = useState<Record<string, string>>({});

  const ordering = ensureArray(settings.time_sync_ordering);
  const ntpServers = ensureArray(settings.time_sync_ntp_servers);
  const httpUrls = ensureArray(settings.time_sync_http_urls);

  const availableSources = ALL_SOURCES.filter(s => !ordering.includes(s));

  const addSource = (source: string) => {
    onChange({ time_sync_ordering: [...ordering, source] });
    setErrors(prev => ({ ...prev, ordering: "" }));
  };

  const removeSource = (index: number) => {
    const newOrdering = ordering.filter((_, i) => i !== index);
    onChange({ time_sync_ordering: newOrdering });
    // Re-validate dependent fields
    validateNtpServers(ntpServers, newOrdering);
    validateHttpUrls(httpUrls, newOrdering);
  };

  const validateNtpServers = (servers: string[], ord: string[]) => {
    if (ord.includes("ntp_user_provided") && servers.every(s => !s.trim())) {
      setErrors(prev => ({ ...prev, ntpServers: $at("At least one NTP server is required") }));
      return false;
    }
    setErrors(prev => ({ ...prev, ntpServers: "" }));
    return true;
  };

  const validateHttpUrls = (urls: string[], ord: string[]) => {
    if (ord.includes("http_user_provided") && urls.every(u => !u.trim())) {
      setErrors(prev => ({ ...prev, httpUrls: $at("At least one HTTP URL is required") }));
      return false;
    }
    setErrors(prev => ({ ...prev, httpUrls: "" }));
    return true;
  };

  const isValidUrl = (value: string) => {
    try {
      const u = new URL(value);
      return u.protocol === "http:" || u.protocol === "https:";
    } catch {
      return false;
    }
  };

  return (
    <GridCard>
      <div className="p-4 space-y-4 text-black dark:text-white">
        <h3 className="text-base font-bold text-slate-900 dark:text-white">
          {$at("Custom Time Synchronization")}
        </h3>

        {/* Source ordering */}
        <div className="flex w-full flex-col gap-1">
          <FieldLabel
            label={$at("Source Ordering")}
            description={$at("Time sources tried in order. Drag to reorder (first = highest priority).")}
          />
          {ordering.length > 0 && (
            <div className="flex flex-wrap gap-1 pb-2">
              {ordering.map((source, idx) => (
                <span
                  key={`source-${idx}`}
                  className="inline-flex items-center rounded-md bg-blue-100 px-1 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900/40 dark:text-blue-200"
                >
                  <span className="px-1">{TIME_SOURCE_LABELS[source] || source}</span>
                  <Button
                    size="XS"
                    theme="blank"
                    type="button"
                    onClick={() => removeSource(idx)}
                    LeadingIcon={LuX}
                  />
                </span>
              ))}
            </div>
          )}
          {availableSources.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {availableSources.map(source => (
                <Button
                  key={source}
                  size="XS"
                  theme="light"
                  text={`+ ${TIME_SOURCE_LABELS[source] || source}`}
                  onClick={() => addSource(source)}
                />
              ))}
            </div>
          )}
          {errors.ordering && <FieldError error={errors.ordering} />}
        </div>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {/* Custom NTP servers */}
          <div className="space-y-3">
            <FieldLabel label={$at("Custom NTP Servers")} />
            {ntpServers.map((server, idx) => (
              <div key={idx} className="flex items-start gap-x-2">
                <div className="flex-1">
                  <InputFieldWithLabel
                    label=""
                    type="text"
                    size="SM"
                    placeholder="pool.ntp.org"
                    value={server}
                    onChange={e => {
                      const newServers = [...ntpServers];
                      newServers[idx] = e.target.value;
                      onChange({ time_sync_ntp_servers: newServers });
                      validateNtpServers(newServers, ordering);
                    }}
                  />
                </div>
                <div className="shrink-0 mt-6">
                  <Button
                    size="SM"
                    theme="light"
                    type="button"
                    onClick={() => {
                      const newServers = ntpServers.filter((_, i) => i !== idx);
                      onChange({ time_sync_ntp_servers: newServers });
                      validateNtpServers(newServers, ordering);
                    }}
                    LeadingIcon={LuX}
                  />
                </div>
              </div>
            ))}
            {errors.ntpServers && <FieldError error={errors.ntpServers} />}
            <Button
              size="SM"
              theme="light"
              text={$at("Add NTP Server")}
              LeadingIcon={LuPlus}
              type="button"
              disabled={ntpServers.some(s => !s.trim())}
              onClick={() => {
                onChange({ time_sync_ntp_servers: [...ntpServers, ""] });
              }}
            />
          </div>

          {/* Custom HTTP URLs */}
          <div className="space-y-3">
            <FieldLabel label={$at("Custom HTTP URLs")} />
            {httpUrls.map((url, idx) => (
              <div key={idx} className="flex items-start gap-x-2">
                <div className="flex-1">
                  <InputFieldWithLabel
                    label=""
                    type="text"
                    size="SM"
                    placeholder="http://www.gstatic.com/generate_204"
                    value={url}
                    onChange={e => {
                      const newUrls = [...httpUrls];
                      newUrls[idx] = e.target.value;
                      onChange({ time_sync_http_urls: newUrls });
                      if (!isValidUrl(e.target.value)) {
                        setErrors(prev => ({
                          ...prev,
                          [`httpUrl_${idx}`]: $at("Must be a valid http:// or https:// URL"),
                        }));
                      } else {
                        setErrors(prev => {
                          const next = { ...prev };
                          delete next[`httpUrl_${idx}`];
                          return next;
                        });
                      }
                      validateHttpUrls(newUrls, ordering);
                    }}
                    error={errors[`httpUrl_${idx}`]}
                  />
                </div>
                <div className="shrink-0 mt-6">
                  <Button
                    size="SM"
                    theme="light"
                    type="button"
                    onClick={() => {
                      const newUrls = httpUrls.filter((_, i) => i !== idx);
                      onChange({ time_sync_http_urls: newUrls });
                      validateHttpUrls(newUrls, ordering);
                    }}
                    LeadingIcon={LuX}
                  />
                </div>
              </div>
            ))}
            {errors.httpUrls && <FieldError error={errors.httpUrls} />}
            <Button
              size="SM"
              theme="light"
              text={$at("Add HTTP URL")}
              LeadingIcon={LuPlus}
              type="button"
              disabled={httpUrls.some(u => !u.trim())}
              onClick={() => {
                onChange({ time_sync_http_urls: [...httpUrls, ""] });
              }}
            />
          </div>
        </div>

        {/* Disable fallback */}
        <div className="flex items-center justify-between">
          <div>
            <FieldLabel
              label={$at("Disable Fallback")}
              description={$at("When enabled, the device will not fall back to other time sources if the primary source fails.")}
            />
          </div>
          <input
            type="checkbox"
            className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
            checked={settings.time_sync_disable_fallback ?? false}
            onChange={e => onChange({ time_sync_disable_fallback: e.target.checked })}
          />
        </div>

        {/* Parallel queries */}
        <div>
          <InputFieldWithLabel
            type="number"
            size="SM"
            label={$at("Parallel Queries")}
            description={$at("Number of time sources queried simultaneously.")}
            placeholder="4"
            value={String(settings.time_sync_parallel ?? 4)}
            min="1"
            onChange={e => {
              const val = parseInt(e.target.value, 10);
              if (!isNaN(val) && val > 0) {
                onChange({ time_sync_parallel: val });
              }
            }}
          />
        </div>
      </div>
    </GridCard>
  );
}
