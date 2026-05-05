"use client";

import { use } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { payments } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { formatAmount, formatDate } from "@/lib/types";

interface Props {
  params: Promise<{ id: string }>;
}

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between border-b border-gray-50 py-3 last:border-0">
      <span className="text-sm font-medium text-gray-500">{label}</span>
      <span className="ml-4 text-sm text-gray-900">{value}</span>
    </div>
  );
}

export default function PaymentDetailPage({ params }: Props) {
  const { id } = use(params);
  const { token } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();

  const { data: payment, isLoading } = useQuery({
    queryKey: ["payment", id],
    queryFn: () => payments.get(token!, id),
    enabled: !!token,
  });

  const capture = useMutation({
    mutationFn: () => payments.capture(token!, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["payment", id] }),
  });

  const refund = useMutation({
    mutationFn: () => payments.refund(token!, id, "Merchant-initiated refund"),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["payment", id] }),
  });

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Spinner size="lg" />
      </div>
    );
  }

  if (!payment) {
    return (
      <div className="text-center text-gray-500">Payment not found.</div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <button
          onClick={() => router.back()}
          className="text-sm text-gray-400 hover:text-gray-700"
        >
          ← Back
        </button>
        <h1 className="text-xl font-bold text-gray-900">Payment Detail</h1>
      </div>

      {/* Card */}
      <div className="rounded-2xl border border-gray-100 bg-white p-6 shadow-sm">
        <div className="mb-4 flex items-center justify-between">
          <p className="font-mono text-xs text-gray-400">{payment.id}</p>
          <Badge status={payment.status} />
        </div>

        <DetailRow label="Amount"      value={formatAmount(payment.amount, payment.currency)} />
        <DetailRow label="Method"      value={<span className="capitalize">{payment.method}</span>} />
        <DetailRow label="Currency"    value={payment.currency} />
        <DetailRow label="Created"     value={formatDate(payment.created_at)} />
        <DetailRow label="Updated"     value={formatDate(payment.updated_at)} />
        {payment.failure_reason && (
          <DetailRow
            label="Failure reason"
            value={<span className="text-red-600">{payment.failure_reason}</span>}
          />
        )}
      </div>

      {/* Actions */}
      <div className="flex gap-3">
        {payment.status === "authorized" && (
          <button
            onClick={() => capture.mutate()}
            disabled={capture.isPending}
            className="flex items-center gap-2 rounded-xl bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-indigo-700 disabled:opacity-60"
          >
            {capture.isPending ? <Spinner size="sm" /> : null}
            Capture
          </button>
        )}
        {payment.status === "completed" && (
          <button
            onClick={() => refund.mutate()}
            disabled={refund.isPending}
            className="flex items-center gap-2 rounded-xl border border-red-200 bg-red-50 px-5 py-2.5 text-sm font-semibold text-red-700 hover:bg-red-100 disabled:opacity-60"
          >
            {refund.isPending ? <Spinner size="sm" /> : null}
            Refund
          </button>
        )}
      </div>

      {/* Mutation errors */}
      {(capture.error || refund.error) && (
        <p className="text-sm text-red-600">
          {String((capture.error ?? refund.error))}
        </p>
      )}
    </div>
  );
}
