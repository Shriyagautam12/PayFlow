"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Spinner } from "@/components/ui/Spinner";
import { StatCard } from "@/components/ui/StatCard";
import { Table } from "@/components/ui/Table";
import { wallet } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { formatAmount, formatDate, type LedgerEntry } from "@/lib/types";

export default function WalletPage() {
  const { token } = useAuth();
  const queryClient = useQueryClient();

  const [page, setPage] = useState(1);
  const [showPayout, setShowPayout] = useState(false);
  const [destination, setDestination] = useState("");
  const [payoutAmount, setPayoutAmount] = useState("");
  const [payoutError, setPayoutError] = useState("");

  const { data: walletData, isLoading: walletLoading } = useQuery({
    queryKey: ["wallet"],
    queryFn: () => wallet.get(token!),
    enabled: !!token,
  });

  const { data: ledgerData, isLoading: ledgerLoading } = useQuery({
    queryKey: ["ledger", page],
    queryFn: () => wallet.ledger(token!, page, 20),
    enabled: !!token,
  });

  const payout = useMutation({
    mutationFn: () =>
      wallet.payout(token!, {
        amount: Math.round(parseFloat(payoutAmount) * 100), // rupees → paise
        currency: walletData?.currency ?? "INR",
        destination,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["wallet"] });
      queryClient.invalidateQueries({ queryKey: ["ledger"] });
      setShowPayout(false);
      setDestination("");
      setPayoutAmount("");
    },
    onError: (e) => setPayoutError(String(e)),
  });

  const ledgerColumns = [
    {
      key: "type",
      header: "Type",
      render: (e: LedgerEntry) => (
        <span
          className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${
            e.type === "credit"
              ? "bg-green-100 text-green-700"
              : "bg-red-100 text-red-600"
          }`}
        >
          {e.type === "credit" ? "+" : "−"} {e.type}
        </span>
      ),
    },
    {
      key: "amount",
      header: "Amount",
      render: (e: LedgerEntry) => (
        <span className={`font-semibold ${e.type === "credit" ? "text-green-700" : "text-red-600"}`}>
          {e.type === "credit" ? "+" : "−"}{formatAmount(e.amount)}
        </span>
      ),
    },
    {
      key: "description",
      header: "Description",
      render: (e: LedgerEntry) => <span className="text-gray-600">{e.description}</span>,
    },
    {
      key: "created_at",
      header: "Date",
      render: (e: LedgerEntry) => <span className="text-gray-400">{formatDate(e.created_at)}</span>,
    },
  ];

  if (walletLoading) {
    return <div className="flex h-64 items-center justify-center"><Spinner size="lg" /></div>;
  }

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Wallet</h1>
        <button
          onClick={() => setShowPayout(true)}
          className="rounded-xl bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-indigo-700"
        >
          Payout
        </button>
      </div>

      {/* Balance card */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <StatCard
          label="Available Balance"
          value={formatAmount(walletData?.balance ?? 0)}
          sub={walletData?.currency}
        />
      </div>

      {/* Payout form */}
      {showPayout && (
        <div className="rounded-2xl border border-gray-100 bg-white p-6 shadow-sm">
          <h2 className="mb-4 text-base font-semibold text-gray-900">Initiate Payout</h2>
          <div className="space-y-4">
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">
                Amount (₹)
              </label>
              <input
                type="number"
                min="1"
                value={payoutAmount}
                onChange={(e) => setPayoutAmount(e.target.value)}
                placeholder="500.00"
                className="w-full rounded-xl border border-gray-200 px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-300"
              />
            </div>
            <div>
              <label className="mb-1 block text-sm font-medium text-gray-700">
                Destination (bank/UPI)
              </label>
              <input
                type="text"
                value={destination}
                onChange={(e) => setDestination(e.target.value)}
                placeholder="merchant@upi or IFSC/account"
                className="w-full rounded-xl border border-gray-200 px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-300"
              />
            </div>
            {payoutError && <p className="text-sm text-red-600">{payoutError}</p>}
            <div className="flex gap-3">
              <button
                onClick={() => payout.mutate()}
                disabled={payout.isPending || !payoutAmount || !destination}
                className="flex items-center gap-2 rounded-xl bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white disabled:opacity-60 hover:bg-indigo-700"
              >
                {payout.isPending && <Spinner size="sm" />}
                Confirm
              </button>
              <button
                onClick={() => setShowPayout(false)}
                className="rounded-xl border border-gray-200 px-5 py-2.5 text-sm font-semibold text-gray-600 hover:bg-gray-50"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Ledger table */}
      <div>
        <h2 className="mb-4 text-base font-semibold text-gray-700">Transaction History</h2>
        {ledgerLoading ? (
          <div className="flex h-40 items-center justify-center"><Spinner /></div>
        ) : (
          <>
            <Table
              columns={ledgerColumns}
              rows={ledgerData?.entries ?? []}
              emptyMessage="No ledger entries yet."
            />
            {(ledgerData?.total_pages ?? 1) > 1 && (
              <div className="mt-4 flex items-center justify-between">
                <p className="text-sm text-gray-500">
                  Page {ledgerData?.page} of {ledgerData?.total_pages}
                </p>
                <div className="flex gap-2">
                  <button disabled={page <= 1} onClick={() => setPage((p) => p - 1)}
                    className="rounded-xl border border-gray-200 bg-white px-4 py-2 text-sm disabled:opacity-40 hover:bg-gray-50">
                    ← Prev
                  </button>
                  <button disabled={page >= (ledgerData?.total_pages ?? 1)} onClick={() => setPage((p) => p + 1)}
                    className="rounded-xl border border-gray-200 bg-white px-4 py-2 text-sm disabled:opacity-40 hover:bg-gray-50">
                    Next →
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
