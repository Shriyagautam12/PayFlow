"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Spinner } from "@/components/ui/Spinner";
import { webhooks } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { formatDate, type EventType, type Webhook } from "@/lib/types";

const EVENT_TYPES: EventType[] = [
  "payment.initiated",
  "payment.authorized",
  "payment.completed",
  "payment.failed",
  "payment.refunded",
];

export default function WebhooksPage() {
  const { token } = useAuth();
  const queryClient = useQueryClient();

  const [url, setUrl] = useState("");
  const [eventType, setEventType] = useState<EventType>("payment.completed");
  const [newSecret, setNewSecret] = useState<string | null>(null);
  const [formError, setFormError] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["webhooks"],
    queryFn: () => webhooks.list(token!),
    enabled: !!token,
  });

  const register = useMutation({
    mutationFn: () => webhooks.register(token!, { url, event_type: eventType }),
    onSuccess: (res) => {
      setNewSecret(res.secret);
      setUrl("");
      queryClient.invalidateQueries({ queryKey: ["webhooks"] });
    },
    onError: (e) => setFormError(String(e)),
  });

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-bold text-gray-900">Webhooks</h1>

      {/* Register form */}
      <div className="rounded-2xl border border-gray-100 bg-white p-6 shadow-sm">
        <h2 className="mb-4 text-base font-semibold text-gray-900">Register endpoint</h2>
        <div className="space-y-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Endpoint URL
            </label>
            <input
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://your-server.com/webhooks/payflow"
              className="w-full rounded-xl border border-gray-200 px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-300"
            />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">
              Event type
            </label>
            <select
              value={eventType}
              onChange={(e) => setEventType(e.target.value as EventType)}
              className="w-full rounded-xl border border-gray-200 bg-white px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-300"
            >
              {EVENT_TYPES.map((t) => (
                <option key={t} value={t}>{t}</option>
              ))}
            </select>
          </div>
          {formError && <p className="text-sm text-red-600">{formError}</p>}
          <button
            onClick={() => { setFormError(""); register.mutate(); }}
            disabled={register.isPending || !url}
            className="flex items-center gap-2 rounded-xl bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white disabled:opacity-60 hover:bg-indigo-700"
          >
            {register.isPending && <Spinner size="sm" />}
            Register
          </button>
        </div>

        {/* Show secret once */}
        {newSecret && (
          <div className="mt-4 rounded-xl bg-amber-50 border border-amber-200 p-4">
            <p className="mb-1 text-sm font-semibold text-amber-800">
              ⚠️ Save this secret — it will never be shown again
            </p>
            <code className="block break-all text-xs text-amber-900">{newSecret}</code>
            <p className="mt-2 text-xs text-amber-700">
              Use this to verify <code>X-PayFlow-Signature</code> on incoming requests.
            </p>
            <button
              onClick={() => setNewSecret(null)}
              className="mt-3 text-xs text-amber-600 underline"
            >
              I've saved it — dismiss
            </button>
          </div>
        )}
      </div>

      {/* Registered webhooks */}
      <div>
        <h2 className="mb-4 text-base font-semibold text-gray-700">Registered endpoints</h2>
        {isLoading ? (
          <div className="flex h-40 items-center justify-center"><Spinner /></div>
        ) : (data?.webhooks ?? []).length === 0 ? (
          <p className="text-sm text-gray-400">No webhooks registered yet.</p>
        ) : (
          <div className="space-y-3">
            {(data?.webhooks ?? []).map((wh: Webhook) => (
              <div
                key={wh.id}
                className="flex items-start justify-between rounded-2xl border border-gray-100 bg-white p-5 shadow-sm"
              >
                <div className="space-y-1">
                  <p className="text-sm font-medium text-gray-900 break-all">{wh.url}</p>
                  <span className="inline-flex items-center rounded-full bg-indigo-50 px-2.5 py-0.5 text-xs font-medium text-indigo-700">
                    {wh.event_type}
                  </span>
                </div>
                <div className="ml-4 flex flex-col items-end gap-1">
                  <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${wh.active ? "bg-green-100 text-green-700" : "bg-gray-100 text-gray-500"}`}>
                    {wh.active ? "active" : "inactive"}
                  </span>
                  <span className="text-xs text-gray-400">{formatDate(wh.created_at)}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
