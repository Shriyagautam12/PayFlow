"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { formatAmount } from "@/lib/types";

const UPI_TIMEOUT_SECONDS = 300; // 5 minutes — standard UPI QR window

function formatCountdown(seconds: number) {
  const m = Math.floor(seconds / 60).toString().padStart(2, "0");
  const s = (seconds % 60).toString().padStart(2, "0");
  return `${m}:${s}`;
}

// Build a UPI deep link. In production this would come from the backend.
function buildUPILink(paymentId: string, amount: number) {
  const paise = amount;
  const rupees = (paise / 100).toFixed(2);
  return `upi://pay?pa=payflow@upi&pn=PayFlow&am=${rupees}&cu=INR&tn=${paymentId}`;
}

// Placeholder QR — in production generate a real QR from the UPI link server-side.
function QRPlaceholder({ value }: { value: string }) {
  return (
    <div className="flex h-48 w-48 flex-col items-center justify-center rounded-2xl border-2 border-dashed border-indigo-300 bg-indigo-50">
      <p className="text-xs text-indigo-400 text-center px-2 break-all">{value.slice(0, 60)}…</p>
      <p className="mt-2 text-xs text-gray-400">QR renders here</p>
    </div>
  );
}

export default function UPIPage() {
  const router = useRouter();
  const searchParams = useSearchParams();

  const paymentId = searchParams.get("payment_id") ?? "";
  const amount = Number(searchParams.get("amount") ?? 0);

  const [secondsLeft, setSecondsLeft] = useState(UPI_TIMEOUT_SECONDS);
  const [expired, setExpired] = useState(false);

  const upiLink = buildUPILink(paymentId, amount);

  // Countdown timer
  useEffect(() => {
    if (secondsLeft <= 0) {
      setExpired(true);
      return;
    }
    const t = setTimeout(() => setSecondsLeft((s) => s - 1), 1000);
    return () => clearTimeout(t);
  }, [secondsLeft]);

  // Poll payment status every 3s — redirect to status page once backend confirms
  useEffect(() => {
    if (!paymentId || expired) return;

    const interval = setInterval(() => {
      fetch(`${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/v1"}/payment/${paymentId}`)
        .then((r) => r.json())
        .then((p) => {
          if (p.status === "completed" || p.status === "failed") {
            clearInterval(interval);
            router.push(`/checkout/status?payment_id=${paymentId}`);
          }
        })
        .catch(() => {}); // network errors are non-fatal during polling
    }, 3000);

    return () => clearInterval(interval);
  }, [paymentId, expired, router]);

  const urgency =
    secondsLeft <= 30
      ? "text-red-600"
      : secondsLeft <= 60
        ? "text-amber-600"
        : "text-indigo-600";

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-sm space-y-6 text-center">
        {/* Brand */}
        <div>
          <span className="text-2xl font-black text-indigo-600">Pay</span>
          <span className="text-2xl font-black text-gray-900">Flow</span>
        </div>

        <div className="rounded-2xl border border-gray-100 bg-white p-8 shadow-sm space-y-5">
          <div>
            <p className="text-sm text-gray-500">Scan & pay</p>
            <p className="mt-1 text-3xl font-black text-gray-900">
              {formatAmount(amount)}
            </p>
          </div>

          {/* QR */}
          <div className="flex justify-center">
            {expired ? (
              <div className="flex h-48 w-48 items-center justify-center rounded-2xl bg-red-50 border border-red-200">
                <p className="text-sm font-semibold text-red-600">QR Expired</p>
              </div>
            ) : (
              <QRPlaceholder value={upiLink} />
            )}
          </div>

          {/* Timer */}
          {!expired ? (
            <div>
              <p className={`text-2xl font-black tabular-nums ${urgency}`}>
                {formatCountdown(secondsLeft)}
              </p>
              <p className="mt-0.5 text-xs text-gray-400">time remaining</p>
            </div>
          ) : (
            <div className="space-y-3">
              <p className="text-sm text-red-600 font-medium">This QR has expired.</p>
              <button
                onClick={() => router.back()}
                className="rounded-xl bg-indigo-600 px-6 py-2.5 text-sm font-semibold text-white hover:bg-indigo-700"
              >
                Try again
              </button>
            </div>
          )}

          {/* UPI deep link for mobile */}
          {!expired && (
            <a
              href={upiLink}
              className="block rounded-xl border border-indigo-200 px-4 py-2.5 text-sm font-medium text-indigo-700 hover:bg-indigo-50"
            >
              Open UPI app directly
            </a>
          )}
        </div>

        <p className="text-xs text-gray-400">
          Waiting for payment confirmation…
        </p>
      </div>
    </div>
  );
}
