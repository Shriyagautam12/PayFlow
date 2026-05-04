import type {
  AuthResponse,
  LedgerResponse,
  ListPaymentsParams,
  ListPaymentsResponse,
  ListWebhooksResponse,
  LoginRequest,
  Payment,
  PayoutRequest,
  RegisterRequest,
  RegisterWebhookRequest,
  RegisterWebhookResponse,
  WalletResponse,
} from "./types";

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/v1";

// ── Core fetch wrapper ────────────────────────────────────────────────────────

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(
  path: string,
  options: RequestInit = {},
  token?: string,
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${BASE_URL}${path}`, { ...options, headers });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, body.error ?? "Request failed");
  }

  // 204 No Content
  if (res.status === 204) return undefined as T;

  return res.json() as Promise<T>;
}

// ── Auth ──────────────────────────────────────────────────────────────────────

export const auth = {
  register: (body: RegisterRequest) =>
    request<AuthResponse>("/auth/register", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  login: (body: LoginRequest) =>
    request<AuthResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  refresh: () =>
    request<AuthResponse>("/auth/refresh", {
      method: "POST",
      credentials: "include", // sends refresh token cookie
    }),

  logout: (token: string) =>
    request<void>("/auth/logout", {
      method: "POST",
      credentials: "include",
    }, token),
};

// ── Payments ──────────────────────────────────────────────────────────────────

export const payments = {
  list: (token: string, params: ListPaymentsParams = {}) => {
    const qs = new URLSearchParams();
    if (params.status)    qs.set("status",    params.status);
    if (params.method)    qs.set("method",    params.method);
    if (params.page)      qs.set("page",      String(params.page));
    if (params.page_size) qs.set("page_size", String(params.page_size));
    const query = qs.toString() ? `?${qs}` : "";
    return request<ListPaymentsResponse>(`/payment${query}`, {}, token);
  },

  get: (token: string, id: string) =>
    request<Payment>(`/payment/${id}`, {}, token),

  capture: (token: string, id: string) =>
    request<Payment>(`/payment/${id}/capture`, { method: "POST" }, token),

  refund: (token: string, id: string, reason?: string) =>
    request<Payment>(`/payment/${id}/refund`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    }, token),

  initiate: (
    body: { amount: number; currency: string; method: string; metadata?: Record<string, string> },
    idempotencyKey: string,
    token?: string,
  ) =>
    request<Payment>("/payment", {
      method: "POST",
      body: JSON.stringify(body),
      headers: { "Idempotency-Key": idempotencyKey },
    }, token),
};

// ── Wallet ────────────────────────────────────────────────────────────────────

export const wallet = {
  get: (token: string) =>
    request<WalletResponse>("/wallet", {}, token),

  ledger: (token: string, page = 1, pageSize = 20) =>
    request<LedgerResponse>(
      `/wallet/ledger?page=${page}&page_size=${pageSize}`,
      {},
      token,
    ),

  payout: (token: string, body: PayoutRequest) =>
    request<{ payout_id: string; status: string; new_balance: number }>(
      "/wallet/payout",
      { method: "POST", body: JSON.stringify(body) },
      token,
    ),
};

// ── Webhooks ──────────────────────────────────────────────────────────────────

export const webhooks = {
  register: (token: string, body: RegisterWebhookRequest) =>
    request<RegisterWebhookResponse>("/webhook", {
      method: "POST",
      body: JSON.stringify(body),
    }, token),

  list: (token: string) =>
    request<ListWebhooksResponse>("/webhooks", {}, token),
};

// ── SSE — real-time payment status ────────────────────────────────────────────
// Returns an EventSource. Caller is responsible for closing it.
export function subscribeToPaymentStatus(
  paymentId: string,
  onStatus: (status: string) => void,
  onError?: (e: Event) => void,
): EventSource {
  const es = new EventSource(
    `${BASE_URL}/payment/${paymentId}/status/stream`,
  );
  es.onmessage = (e) => {
    try {
      const data = JSON.parse(e.data);
      onStatus(data.status);
    } catch {
      // ignore malformed frames
    }
  };
  if (onError) es.onerror = onError;
  return es;
}

export { ApiError };
