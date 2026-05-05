"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { subscribeToPaymentStatus } from "@/lib/api";
import { Spinner } from "@/components/ui/Spinner";
import type { PaymentStatus } from "@/lib/types";

const TERMINAL: PaymentStatus[] = ["completed", "failed", "refunded"];

function StatusIcon({ status }: { status: PaymentStatus | "waiting" }) {
  if (status === "waiting" || status === "pending" || status === "initiated" || status === "authorized") {
    return (
      <div className="flex h-20 w-20 items-center justify-center rounded-full bg-indigo-50">
        <Spinner size="lg" />
      </div>
    );
  }
  if (status === "completed") {
    return (
      <div className="flex h-20 w-20 items-center justify-center rounded-full bg-green-100">
        <span className="text-4xl">✓</span>
      </div>
    );
  }
  if (status === "refunded") {
    return (
      <div className="flex h-20 w-20 items-center justify-center rounded-full bg-purple-100">
        <span className="text-4xl">↩</span>
      </div>
    );
  }
  // failed
  return (
    <div className="flex h-20 w-20 items-center justify-center rounded-full bg-red-100">
      <span className="text-4xl">✕</span>
    </div>
  );
}

const statusMessage: Record<PaymentStatus | "waiting", { title: string; sub: string }> = {
  waiting:    { title: "Waiting…",            sub: "Connecting to payment status" },
  initiated:  { title: "Initiating…",         sub: "Setting up your payment" },
  pending:    { title: "Processing…",          sub: "Your payment is being processed" },
  authorized: { title: "Authorized",           sub: "Almost there — completing capture" },
  completed:  { title: "Payment successful!",  sub: "Your payment was completed" },
  failed:     { title: "Payment failed",       sub: "Please try again or use a different method" },
  refunded:   { title: "Payment refunded",     sub: "Amount will reflect in 2–5 business days" },
};

export default function StatusPage() {
  const searchParams = useSearchParams();
  const router = useRouter();

  const paymentId = searchParams.get("payment_id") ?? "";
  const [status, setStatus] = useState<PaymentStatus | "waiting">("waiting");

  useEffect(() => {
    if (!paymentId) return;

    const es = subscribeToPaymentStatus(
      paymentId,
      (newStatus) => {
        setStatus(newStatus as PaymentStatus);
      },
      () => {
        // SSE error — fall back to polling
        setStatus((prev) => prev);
      },
    );

    return () => es.close();
  }, [paymentId]);

  const isTerminal =
    status !== "waiting" && TERMINAL.includes(status as PaymentStatus);

  const msg = statusMessage[status];

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-sm text-center space-y-6">
        {/* Brand */}
        <div>
          <span className="text-2xl font-black text-indigo-600">Pay</span>
          <span className="text-2xl font-black text-gray-900">Flow</span>
        </div>

        <div className="rounded-2xl border border-gray-100 bg-white p-10 shadow-sm space-y-6">
          {/* Icon */}
          <div className="flex justify-center">
            <StatusIcon status={status} />
          </div>

          {/* Message */}
          <div>
            <h1 className="text-xl font-bold text-gray-900">{msg.title}</h1>
            <p className="mt-1 text-sm text-gray-500">{msg.sub}</p>
          </div>

          {/* Payment ID */}
          {paymentId && (
            <p className="font-mono text-xs text-gray-300">
              {paymentId.slice(0, 8)}…
            </p>
          )}

          {/* Actions on terminal states */}
          {isTerminal && (
            <div className="space-y-3">
              {status === "completed" && (
                <button
                  onClick={() => router.push("/")}
                  className="w-full rounded-xl bg-green-600 py-3 text-sm font-semibold text-white hover:bg-green-700"
                >
                  Done
                </button>
              )}
              {status === "failed" && (
                <button
                  onClick={() => router.back()}
                  className="w-full rounded-xl bg-indigo-600 py-3 text-sm font-semibold text-white hover:bg-indigo-700"
                >
                  Try again
                </button>
              )}
              {status === "refunded" && (
                <button
                  onClick={() => router.push("/")}
                  className="w-full rounded-xl border border-gray-200 py-3 text-sm font-semibold text-gray-600 hover:bg-gray-50"
                >
                  Return to home
                </button>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
