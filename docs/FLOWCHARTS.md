# PayFlow — System Flowcharts


---

## 1. Payment Lifecycle — State Machine

```mermaid
stateDiagram-v2
    [*] --> INITIATED : POST /payment
    INITIATED --> PENDING : auto (fraud check)
    INITIATED --> FAILED : validation error

    PENDING --> AUTHORIZED : gateway callback
    PENDING --> FAILED : gateway decline

    AUTHORIZED --> COMPLETED : POST /payment/:id/capture
    AUTHORIZED --> FAILED : capture error

    COMPLETED --> REFUNDED : POST /payment/:id/refund

    FAILED --> [*] : terminal
    REFUNDED --> [*] : terminal

    note right of COMPLETED
        Wallet credited
        before status update
    end note
    note right of REFUNDED
        Wallet debited
        before status update
    end note
```

---

## 2. Initiate Payment — Idempotency Flow

```mermaid
flowchart TD
    A([Client: POST /payment\nIdempotency-Key: xyz]) --> B[Handler validates request]
    B --> C{Redis cache hit?\nkey = idempotency:xyz}

    C -- Yes --> D[Return cached payment\nno DB write]
    D --> Z([200 OK — same response as original])

    C -- No --> E{DB hit?\ncheck idempotency_key column}

    E -- Yes --> F[Cache in Redis\nTTL = 24h]
    F --> Z

    E -- No --> G[New request — create Payment\nstatus = INITIATED]
    G --> H[Write to DB]
    H --> I[Cache result in Redis]
    I --> J[Transition: INITIATED → PENDING\nWHERE status = initiated]
    J --> Z2([201 Created])

    style D fill:#d4edda
    style F fill:#d4edda
    style Z fill:#d4edda
    style Z2 fill:#d4edda
```

---

## 3. Capture Payment — Money Before Status

```mermaid
flowchart TD
    A([POST /payment/:id/capture]) --> B[Fetch payment\nscoped to merchant_id]
    B --> C{State machine:\nAuthorized → Completed?}

    C -- Invalid transition --> D([400 ErrInvalidTransition])

    C -- Valid --> E[wallet.Credit\nSELECT FOR UPDATE locks row]
    E --> F{Credit succeeded?}

    F -- No --> G([500 wallet credit failed])

    F -- Yes --> H[UPDATE payments\nSET status = completed\nWHERE id = ? AND status = authorized]
    H --> I{Rows affected = 1?}

    I -- No\nrace condition --> J[🚨 CRITICAL LOG\nwallet credited but\nstatus not updated]
    J --> K([500 needs reconciliation])

    I -- Yes --> L[Publish to Kafka\npayment.completed event]
    L --> M([200 Payment Completed])

    style E fill:#fff3cd
    style H fill:#fff3cd
    style J fill:#f8d7da
    style M fill:#d4edda
```

---

## 4. Refund Payment

```mermaid
flowchart TD
    A([POST /payment/:id/refund]) --> B[Fetch payment]
    B --> C{status == COMPLETED?}

    C -- No --> D([409 ErrPaymentNotComplete])

    C -- Yes --> E[wallet.Debit\nSELECT FOR UPDATE]
    E --> F{Debit succeeded?}

    F -- No --> G([500 wallet debit failed])

    F -- Yes --> H[UPDATE payments\nSET status = refunded\nfailure_reason = reason]
    H --> I{Update succeeded?}

    I -- No --> J[🚨 CRITICAL LOG\nwallet debited but\nstatus not updated]
    J --> K([500 needs reconciliation])

    I -- Yes --> L[Publish to Kafka\npayment.refunded event]
    L --> M([200 Payment Refunded])

    style E fill:#fff3cd
    style J fill:#f8d7da
    style M fill:#d4edda
```

---

## 5. Wallet Credit / Debit — Atomic DB Transaction

```mermaid
flowchart TD
    A([Credit or Debit called]) --> B[BEGIN TRANSACTION]
    B --> C["SELECT balance FROM wallets\nWHERE merchant_id = ?\nFOR UPDATE\n⟵ row lock acquired"]

    C --> D{Operation type?}

    D -- Credit --> E[UPDATE wallets\nSET balance = balance + amount]
    D -- Debit --> F{balance >= amount?}

    F -- No --> G[ROLLBACK]
    G --> H([ErrInsufficientFunds])

    F -- Yes --> I[UPDATE wallets\nSET balance = balance - amount]

    E --> J[INSERT INTO ledger_entries\ntype=credit, amount, balance_after]
    I --> J

    J --> K[COMMIT]
    K --> L([Return new balance])

    style C fill:#fff3cd
    style G fill:#f8d7da
    style H fill:#f8d7da
    style L fill:#d4edda
```

---

## 6. Auth — Register & Login

```mermaid
flowchart TD
    subgraph Register
        R1([POST /auth/register]) --> R2{Email already\nin DB?}
        R2 -- Yes --> R3([409 ErrEmailTaken])
        R2 -- No --> R4[bcrypt hash\npassword, cost=12]
        R4 --> R5[Generate API key\npf_live_ + 24 random bytes]
        R5 --> R6[INSERT merchant\nstatus=active]
        R6 --> R7[issueTokens]
        R7 --> R8([201 access + refresh tokens])
    end

    subgraph Login
        L1([POST /auth/login]) --> L2{Merchant found\nby email?}
        L2 -- No --> L3([401 ErrInvalidCreds\ngeneric — no email leak])
        L2 -- Yes --> L4{status ==\nsuspended?}
        L4 -- Yes --> L5([403 ErrMerchantSuspended])
        L4 -- No --> L6[bcrypt.CompareHashAndPassword\nconstant-time comparison]
        L6 --> L7{Match?}
        L7 -- No --> L3
        L7 -- Yes --> L8[issueTokens]
        L8 --> L9([200 access + refresh tokens])
    end

    style R3 fill:#f8d7da
    style L3 fill:#f8d7da
    style L5 fill:#f8d7da
    style R8 fill:#d4edda
    style L9 fill:#d4edda
```

---

## 7. Auth — Token Refresh & Google OAuth

```mermaid
flowchart TD
    subgraph Refresh
        RF1([POST /auth/refresh\nrefresh_token cookie]) --> RF2[Hash raw token]
        RF2 --> RF3{Found in DB\nby token_hash?}
        RF3 -- No --> RF4([401 ErrTokenExpired])
        RF3 -- Yes --> RF5{expires_at < now?}
        RF5 -- Yes --> RF6[DELETE from DB]
        RF6 --> RF4
        RF5 -- No --> RF7[Fetch merchant]
        RF7 --> RF8[DELETE old refresh token\none-use rotation]
        RF8 --> RF9[issueTokens — new pair]
        RF9 --> RF10([200 new tokens])
    end

    subgraph Google OAuth
        G1([GET /auth/google/callback]) --> G2{Find by\ngoogle_id?}
        G2 -- Found --> G3{Suspended?}
        G3 -- Yes --> G4([403])
        G3 -- No --> G5[issueTokens]
        G2 -- Not found --> G6{Find by\nemail?}
        G6 -- Found --> G7[Link google_id\nto existing account]
        G7 --> G3
        G6 -- Not found --> G8[Auto-register merchant\nno password set]
        G8 --> G3
        G5 --> G9([200 tokens])
    end

    style RF4 fill:#f8d7da
    style G4 fill:#f8d7da
    style G9 fill:#d4edda
    style RF10 fill:#d4edda
```

---

## 8. Kafka Event Pipeline — End to End

```mermaid
flowchart LR
    subgraph Payment Service
        PS1[Capture / Refund\nsucceeds] --> PS2[events.Producer\n.Publish]
        PS2 --> PS3[kafka.Writer\nkey = PaymentID]
    end

    subgraph Kafka Broker
        PS3 --> K1[Topic: payment-events\nPartitioned by PaymentID\nOrder preserved per payment]
    end

    subgraph Webhook Service
        K1 --> C1[events.Consumer\nGroupID: webhook-service]
        C1 --> C2{Unmarshal\nPaymentEvent?}
        C2 -- Fail --> C3[Commit bad message\nprevent infinite stall]
        C2 -- OK --> C4[webhookSvc.HandleEvent]
        C4 --> C5[Commit offset]
    end

    style K1 fill:#e2e3e5
    style C3 fill:#fff3cd
```

---

## 9. Webhook Delivery — Fan-out & Retry

```mermaid
flowchart TD
    A[HandleEvent receives\npayment.completed] --> B["repo.FindActiveByEventType\n(one DB query)"]
    B --> C{Webhooks\nregistered?}
    C -- None --> D([No-op])
    C -- 1..N --> E[For each webhook\ndeliver in parallel]

    E --> F[Marshal event JSON]
    F --> G["sign payload\nHMAC-SHA256 with webhook.Secret"]
    G --> H["HTTP POST to webhook.URL\nHeaders:\nX-PayFlow-Signature: sha256=...\nX-PayFlow-Event: payment.completed\nX-PayFlow-Delivery: event_id\nTimeout: 10s"]

    H --> I{Response\nstatus < 300?}

    I -- Yes --> J[DeliverySucceeded]
    J --> K[INSERT webhook_delivery\nstatus=succeeded]

    I -- No --> L[DeliveryFailed]
    L --> M{attempt < 5?}

    M -- No --> N[INSERT webhook_delivery\nno nextRetryAt\nmax retries reached]

    M -- Yes --> O["nextRetryAt =\nattempt 1 → +30s\nattempt 2 → +5m\nattempt 3 → +30m\nattempt 4 → +2h\nattempt 5 → +8h"]
    O --> P[INSERT webhook_delivery\nstatus=failed\nnextRetryAt set]

    subgraph Retry Ticker every 60s
        Q[ProcessRetries] --> R["SELECT * FROM webhook_deliveries\nWHERE status=failed\nAND next_retry_at <= now"]
        R --> S[Re-run deliver\nattemptNumber + 1]
    end

    style J fill:#d4edda
    style K fill:#d4edda
    style N fill:#f8d7da
```

---

## 10. HMAC Signature Verification (Merchant Side)

```mermaid
flowchart TD
    A([Webhook POST received\nat merchant server]) --> B[Read raw request body]
    B --> C["Extract header:\nX-PayFlow-Signature: sha256=abc123"]
    C --> D["HMAC-SHA256 the body\nusing your webhook secret"]
    D --> E["Compare:\nyour_digest == abc123?"]

    E -- Match --> F[Request is authentic\nprocess the event]
    E -- No match --> G[Reject — possible tampering\nor wrong secret]

    style F fill:#d4edda
    style G fill:#f8d7da

    note1["⚠️  Always use\nconstant-time comparison\nto prevent timing attacks"]
    E --- note1
```

---

## 11. Full Request Journey — Payment Completed

```mermaid
sequenceDiagram
    participant Client
    participant API as PayFlow API
    participant DB as PostgreSQL
    participant Redis
    participant Kafka
    participant WH as Webhook Service
    participant Merchant as Merchant Webhook URL

    Client->>API: POST /payment (Idempotency-Key: k1)
    API->>Redis: GET idempotency:k1
    Redis-->>API: miss
    API->>DB: SELECT by idempotency_key
    DB-->>API: not found
    API->>DB: INSERT payment (INITIATED)
    API->>Redis: SET idempotency:k1 TTL=24h
    API->>DB: UPDATE status INITIATED→PENDING
    API-->>Client: 201 {payment_id, status: pending}

    Note over API,DB: Gateway authorizes...
    Client->>API: POST /payment/:id/capture
    API->>DB: SELECT payment FOR UPDATE
    API->>DB: Credit wallet (SELECT FOR UPDATE → UPDATE balance → INSERT ledger)
    API->>DB: UPDATE payment AUTHORIZED→COMPLETED
    API->>Kafka: Publish payment.completed {event_id, payment_id, amount}
    API-->>Client: 200 {status: completed}

    Kafka->>WH: Consume payment.completed
    WH->>DB: SELECT webhooks WHERE event_type='payment.completed' AND active=true
    DB-->>WH: [{webhook_id, url, secret}]
    WH->>WH: HMAC-SHA256 sign payload
    WH->>Merchant: POST /your-url\nX-PayFlow-Signature: sha256=...
    Merchant-->>WH: 200 OK
    WH->>DB: INSERT webhook_delivery (succeeded)
```

---

## Key Design Decisions — Quick Reference

| Concept | Where | Why |
|---|---|---|
| Idempotency Redis → DB two-layer | `payment/service.go Initiate` | Redis is fast but expires; DB is the source of truth |
| `WHERE status = ?` in UPDATE | `payment/repository.go UpdateStatus` | Optimistic lock — prevents race conditions without explicit locks |
| Wallet credit **before** status update | `payment/service.go Capture` | Money must move first; status is just a label |
| `SELECT FOR UPDATE` on wallet | `wallet/repository.go` | Prevents two simultaneous captures both reading same balance |
| HMAC-SHA256 on webhooks | `webhook/service.go sign` | Merchant can verify payload hasn't been tampered with |
| Exponential backoff, max 5 attempts | `webhook/service.go retryDelays` | Respects merchant server capacity; stops infinite retries |
| Kafka key = PaymentID | `events/producer.go` | All events for same payment go to same partition → ordered |
| Commit only after handler success | `events/consumer.go` | At-least-once delivery guarantee |
