interface StatCardProps {
  label: string;
  value: string;
  sub?: string;
  trend?: "up" | "down" | "neutral";
}

export function StatCard({ label, value, sub, trend }: StatCardProps) {
  const trendColor =
    trend === "up"
      ? "text-green-600"
      : trend === "down"
        ? "text-red-500"
        : "text-gray-400";

  const trendArrow = trend === "up" ? "↑" : trend === "down" ? "↓" : null;

  return (
    <div className="rounded-2xl border border-gray-100 bg-white p-6 shadow-sm">
      <p className="text-sm font-medium text-gray-500">{label}</p>
      <p className="mt-2 text-3xl font-bold tracking-tight text-gray-900">
        {value}
      </p>
      {sub && (
        <p className={`mt-1 text-sm ${trendColor}`}>
          {trendArrow && <span className="mr-1">{trendArrow}</span>}
          {sub}
        </p>
      )}
    </div>
  );
}
