// ── Mirrors Go backend structs exactly ────────────────────────────────────────
// All amounts are in paise (smallest currency unit). Never float.

export type PaymentStatus =
  | "initiated"
  | "pending"
  | "authorized"
  | "completed"
  | "failed"
  | "refunded";

export type PaymentMethod = "upi" | "card" | "netbanking" | "wallet";

export type EntryType = "credit" | "debit";

export type EventType =
  | "payment.initiated"
  | "payment.authorized"
  | "payment.completed"
  | "payment.failed"
  | "payment.refunded";

// ── Auth ──────────────────────────────────────────────────────────────────────

export interface Merchant {
  id: string;
  name: string;
  email: string;
  api_key: string;
  status: "active" | "suspended";
  created_at: string;
}

export interface AuthResponse {
  merchant: Merchant;
  access_token: string;
  expires_in: number; // seconds
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  name: string;
  email: string;
  password: string;
}

// ── Payments ──────────────────────────────────────────────────────────────────

export interface Payment {
  id: string;
  merchant_id: string;
  amount: number;       // paise
  currency: string;
  status: PaymentStatus;
  method: PaymentMethod;
  failure_reason?: string;
  metadata?: unknown;
  created_at: string;
  updated_at: string;
}

export interface ListPaymentsResponse {
  payments: Payment[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface ListPaymentsParams {
  status?: PaymentStatus;
  method?: PaymentMethod;
  page?: number;
  page_size?: number;
}

// ── Wallet ────────────────────────────────────────────────────────────────────

export interface WalletResponse {
  wallet_id: string;
  merchant_id: string;
  balance: number;    // paise
  currency: string;
  updated_at: string;
}

export interface LedgerEntry {
  id: string;
  wallet_id: string;
  type: EntryType;
  amount: number;     // paise
  description: string;
  ref_id: string;
  created_at: string;
}

export interface LedgerResponse {
  entries: LedgerEntry[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface PayoutRequest {
  amount: number;
  currency: string;
  destination: string;
  description?: string;
}

// ── Webhooks ──────────────────────────────────────────────────────────────────

export interface Webhook {
  id: string;
  url: string;
  event_type: EventType;
  active: boolean;
  created_at: string;
}

export interface RegisterWebhookRequest {
  url: string;
  event_type: EventType;
}

export interface RegisterWebhookResponse extends Webhook {
  secret: string; // shown only once
}

export interface ListWebhooksResponse {
  webhooks: Webhook[];
}

// ── Checkout (customer-facing) ─────────────────────────────────────────────

export interface InitiatePaymentRequest {
  amount: number;
  currency: string;
  method: PaymentMethod;
  metadata?: Record<string, string>;
}

// ── SSE (payment status stream) ───────────────────────────────────────────────

export interface PaymentStatusEvent {
  payment_id: string;
  status: PaymentStatus;
  updated_at: string;
}

// ── UI helpers ────────────────────────────────────────────────────────────────

/** Converts paise to rupee display string e.g. 10050 → "₹100.50" */
export function formatAmount(paise: number, currency = "INR"): string {
  const amount = paise / 100;
  return new Intl.NumberFormat("en-IN", {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
  }).format(amount);
}

/** Formats ISO timestamp to readable local date+time */
export function formatDate(iso: string): string {
  return new Intl.DateTimeFormat("en-IN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(iso));
}

/** Maps PaymentStatus to a Tailwind colour class */
export const statusColor: Record<PaymentStatus, string> = {
  initiated:  "bg-gray-100 text-gray-600",
  pending:    "bg-yellow-100 text-yellow-700",
  authorized: "bg-blue-100 text-blue-700",
  completed:  "bg-green-100 text-green-700",
  failed:     "bg-red-100 text-red-700",
  refunded:   "bg-purple-100 text-purple-700",
};
