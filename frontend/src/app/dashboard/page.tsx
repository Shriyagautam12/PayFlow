"use client";

import { useQuery } from "@tanstack/react-query";
import { RevenueChart } from "@/components/charts/RevenueChart";
import { StatCard } from "@/components/ui/StatCard";
import { Spinner } from "@/components/ui/Spinner";
import { payments, wallet } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { formatAmount, type Payment } from "@/lib/types";

// Build last-14-days revenue chart data from a payments list
function buildChartData(paymentList: Payment[]) {
  const days: Record<string, number> = {};
  const now = new Date();

  for (let i = 13; i >= 0; i--) {
    const d = new Date(now);
    d.setDate(d.getDate() - i);
    const key = d.toLocaleDateString("en-IN", { month: "short", day: "numeric" });
    days[key] = 0;
  }

  for (const p of paymentList) {
    if (p.status !== "completed") continue;
    const key = new Date(p.created_at).toLocaleDateString("en-IN", {
      month: "short",
      day: "numeric",
    });
    if (key in days) days[key] += p.amount;
  }

  return Object.entries(days).map(([date, revenue]) => ({ date, revenue }));
}

export default function OverviewPage() {
  const { token } = useAuth();

  const { data: paymentsData, isLoading: paymentsLoading } = useQuery({
    queryKey: ["payments", "overview"],
    queryFn: () => payments.list(token!, { page_size: 200 }),
    enabled: !!token,
  });

  const { data: walletData, isLoading: walletLoading } = useQuery({
    queryKey: ["wallet"],
    queryFn: () => wallet.get(token!),
    enabled: !!token,
  });

  if (paymentsLoading || walletLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Spinner size="lg" />
      </div>
    );
  }

  const list = paymentsData?.payments ?? [];
  const completed = list.filter((p) => p.status === "completed");
  const failed = list.filter((p) => p.status === "failed");
  const totalRevenue = completed.reduce((s, p) => s + p.amount, 0);
  const successRate =
    list.length > 0
      ? Math.round((completed.length / list.length) * 100)
      : 0;

  const chartData = buildChartData(list);

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Overview</h1>
        <p className="text-sm text-gray-500">Last 200 transactions</p>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard
          label="Total Revenue"
          value={formatAmount(totalRevenue)}
          sub="completed payments"
          trend="up"
        />
        <StatCard
          label="Wallet Balance"
          value={formatAmount(walletData?.balance ?? 0)}
          sub={walletData?.currency ?? "INR"}
        />
        <StatCard
          label="Success Rate"
          value={`${successRate}%`}
          sub={`${failed.length} failed`}
          trend={successRate >= 90 ? "up" : "down"}
        />
        <StatCard
          label="Total Transactions"
          value={String(list.length)}
          sub={`${completed.length} completed`}
        />
      </div>

      {/* Revenue chart */}
      <RevenueChart data={chartData} />
    </div>
  );
}
