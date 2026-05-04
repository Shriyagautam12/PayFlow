import type { PaymentStatus } from "@/lib/types";
import { statusColor } from "@/lib/types";

interface BadgeProps {
  status: PaymentStatus;
}

export function Badge({ status }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${statusColor[status]}`}
    >
      {status}
    </span>
  );
}
