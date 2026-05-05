"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { Table } from "@/components/ui/Table";
import { payments } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import {
  formatAmount,
  formatDate,
  type Payment,
  type PaymentMethod,
  type PaymentStatus,
} from "@/lib/types";

const STATUS_OPTIONS: PaymentStatus[] = [
  "initiated", "pending", "authorized", "completed", "failed", "refunded",
];
const METHOD_OPTIONS: PaymentMethod[] = ["upi", "card", "netbanking", "wallet"];

export default function PaymentsPage() {
  const { token } = useAuth();
  const router = useRouter();

  const [status, setStatus] = useState<PaymentStatus | "">("");
  const [method, setMethod] = useState<PaymentMethod | "">("");
  const [page, setPage] = useState(1);

  const { data, isLoading } = useQuery({
    queryKey: ["payments", status, method, page],
    queryFn: () =>
      payments.list(token!, {
        status: status || undefined,
        method: method || undefined,
        page,
        page_size: 20,
      }),
    enabled: !!token,
  });

  const columns = [
    {
      key: "id",
      header: "Payment ID",
      render: (p: Payment) => (
        <span className="font-mono text-xs text-gray-500">
          {p.id.slice(0, 8)}…
        </span>
      ),
    },
    {
      key: "amount",
      header: "Amount",
      render: (p: Payment) => (
        <span className="font-semibold">{formatAmount(p.amount, p.currency)}</span>
      ),
    },
    {
      key: "method",
      header: "Method",
      render: (p: Payment) => (
        <span className="capitalize text-gray-600">{p.method}</span>
      ),
    },
    {
      key: "status",
      header: "Status",
      render: (p: Payment) => <Badge status={p.status} />,
    },
    {
      key: "created_at",
      header: "Date",
      render: (p: Payment) => (
        <span className="text-gray-500">{formatDate(p.created_at)}</span>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Payments</h1>

      {/* Filters */}
      <div className="flex flex-wrap gap-3">
        <select
          value={status}
          onChange={(e) => { setStatus(e.target.value as PaymentStatus | ""); setPage(1); }}
          className="rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-300"
        >
          <option value="">All statuses</option>
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s} className="capitalize">{s}</option>
          ))}
        </select>

        <select
          value={method}
          onChange={(e) => { setMethod(e.target.value as PaymentMethod | ""); setPage(1); }}
          className="rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-300"
        >
          <option value="">All methods</option>
          {METHOD_OPTIONS.map((m) => (
            <option key={m} value={m} className="capitalize">{m}</option>
          ))}
        </select>
      </div>

      {/* Table */}
      {isLoading ? (
        <div className="flex h-64 items-center justify-center">
          <Spinner />
        </div>
      ) : (
        <>
          <Table
            columns={columns}
            rows={data?.payments ?? []}
            onRowClick={(p) => router.push(`/dashboard/payments/${p.id}`)}
            emptyMessage="No payments found for the selected filters."
          />

          {/* Pagination */}
          {(data?.total_pages ?? 1) > 1 && (
            <div className="flex items-center justify-between">
              <p className="text-sm text-gray-500">
                Page {data?.page} of {data?.total_pages} &nbsp;·&nbsp; {data?.total} total
              </p>
              <div className="flex gap-2">
                <button
                  disabled={page <= 1}
                  onClick={() => setPage((p) => p - 1)}
                  className="rounded-xl border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-600 shadow-sm disabled:opacity-40 hover:bg-gray-50"
                >
                  ← Prev
                </button>
                <button
                  disabled={page >= (data?.total_pages ?? 1)}
                  onClick={() => setPage((p) => p + 1)}
                  className="rounded-xl border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-600 shadow-sm disabled:opacity-40 hover:bg-gray-50"
                >
                  Next →
                </button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
