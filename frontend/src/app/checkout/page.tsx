"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import { Spinner } from "@/components/ui/Spinner";
import { payments } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { formatAmount, type PaymentMethod } from "@/lib/types";

const METHODS: { id: PaymentMethod; label: string; icon: string; desc: string }[] = [
  { id: "upi",        label: "UPI",         icon: "⊕", desc: "Pay via UPI QR or ID" },
  { id: "card",       label: "Card",         icon: "▭", desc: "Credit or debit card" },
  { id: "netbanking", label: "Net Banking",  icon: "⊞", desc: "Pay via your bank" },
  { id: "wallet",     label: "Wallet",       icon: "◎", desc: "PayFlow balance" },
];

function randomKey() {
  return `order_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

export default function CheckoutPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { token } = useAuth();

  // Amount passed as query param in paise e.g. ?amount=50000&currency=INR
  const amount = Number(searchParams.get("amount") ?? 50000);
  const currency = searchParams.get("currency") ?? "INR";

  const [selected, setSelected] = useState<PaymentMethod>("upi");

  const initiate = useMutation({
    mutationFn: () =>
      payments.initiate(
        { amount, currency, method: selected },
        randomKey(),
        token ?? undefined,
      ),
    onSuccess: (payment) => {
      if (selected === "upi") {
        router.push(`/checkout/upi?payment_id=${payment.id}&amount=${amount}`);
      } else {
        router.push(`/checkout/status?payment_id=${payment.id}`);
      }
    },
  });

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-md space-y-6">
        {/* Header */}
        <div className="text-center">
          <span className="text-2xl font-black text-indigo-600">Pay</span>
          <span className="text-2xl font-black text-gray-900">Flow</span>
          <p className="mt-1 text-sm text-gray-500">Complete your payment</p>
        </div>

        {/* Amount */}
        <div className="rounded-2xl border border-gray-100 bg-white p-6 text-center shadow-sm">
          <p className="text-sm text-gray-500">Amount due</p>
          <p className="mt-1 text-4xl font-black text-gray-900">
            {formatAmount(amount, currency)}
          </p>
        </div>

        {/* Method selector */}
        <div className="rounded-2xl border border-gray-100 bg-white p-6 shadow-sm">
          <p className="mb-4 text-sm font-semibold text-gray-700">
            Choose payment method
          </p>
          <div className="space-y-3">
            {METHODS.map((m) => (
              <button
                key={m.id}
                onClick={() => setSelected(m.id)}
                className={`flex w-full items-center gap-4 rounded-xl border p-4 text-left transition-all ${
                  selected === m.id
                    ? "border-indigo-400 bg-indigo-50 shadow-sm"
                    : "border-gray-100 hover:border-gray-200 hover:bg-gray-50"
                }`}
              >
                <span className="text-2xl">{m.icon}</span>
                <div>
                  <p className="text-sm font-semibold text-gray-900">{m.label}</p>
                  <p className="text-xs text-gray-400">{m.desc}</p>
                </div>
                {selected === m.id && (
                  <span className="ml-auto h-2 w-2 rounded-full bg-indigo-600" />
                )}
              </button>
            ))}
          </div>
        </div>

        {/* Pay button */}
        <button
          onClick={() => initiate.mutate()}
          disabled={initiate.isPending}
          className="flex w-full items-center justify-center gap-2 rounded-2xl bg-indigo-600 py-4 text-base font-bold text-white shadow-md hover:bg-indigo-700 disabled:opacity-60"
        >
          {initiate.isPending && <Spinner size="sm" />}
          Pay {formatAmount(amount, currency)}
        </button>

        {initiate.error && (
          <p className="text-center text-sm text-red-600">
            {String(initiate.error)}
          </p>
        )}
      </div>
    </div>
  );
}
