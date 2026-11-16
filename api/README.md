# Stock Portfolio API — Backend

Backend API untuk Stock Portfolio Tracker — mengelola data broker, saham, dan portofolio investasi.

## Tech Stack

- **Go** + **Gin** (HTTP framework)
- **GORM** (ORM) + **PostgreSQL**
- **Rate Limiting** (100 req/min)
- **CORS** enabled

## Getting Started

### Prerequisites

- Go 1.21+
- PostgreSQL

### Setup

```bash
cd backend

# Copy env and configure
cp .env.example .env
# Edit .env with your database credentials

# Run air
air 
```

Server runs on `http://localhost:8080`

### Environment Variables

```env
PORT=8080
ENV=development

# Database
DB_USER=postgres
DB_PASS=your_password
DB_HOST=localhost
DB_PORT=5432
DB_NAME=stock_api
```

---

## API Endpoints

### Health Check

| Method | Endpoint | Description |
|--------|----------|:-----------:|
| `GET` | `/health` | API health check |

### Stock (Saham)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/stocks` | Get All stock data ; Param `?limit=$2` to get only limited data |
| `GET` | `/stock/:ticker` | Get stock data by ticker (supports comma-separated: `BBRI,ASII`) |
| `POST` | `/stock` | Add new stock holding |
| `PUT` | `/stock/:ticker` | Update stock holding (tambah lot + recalculate avg price) |
| `DELETE` | `/stock/:ticker` | Delete stock holding (supports comma-separated) |

### Broker (Sekuritas)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/brokers` | Get all brokers |
| `GET` | `/broker/:name` | Get broker details with stock holdings (supports comma-separated) |
| `POST` | `/broker` | Add new broker |
| `PUT` | `/broker/:id` | Update broker data |
| `DELETE` | `/broker/:name` | Delete broker (supports comma-separated) |
| `POST` | `/broker/transfer-cash` | Transfer cash between brokers |

### Cash balance rules and cutover

Cash is a manually initialized balance, not the result of replaying imported history.

- Creating a broker with `cash` initializes both `cash` and `cash_base` to that amount, with `cash_base_as_of = users.created_at`. Brokers discovered by imports start at zero with the same registration cutoff.
- A trade or cash mutation changes cash only when its **event date is strictly after** `max(users.created_at, brokers.cash_base_as_of)`. Equality is history-only. Missing broker cutoff falls back to registration, never all history; a missing registration timestamp fails closed.
- Historical BUY/SELL imports still populate transactions and holdings, but never credit/debit cash. A historical manual BUY also bypasses the insufficient-cash check. Eligible manual BUYs still require enough cash; confirmed imported trades record their actual delta even if the resulting balance is negative.
- Cash-email sync searches only after registration (Gmail epoch-second `after:`), independently of the trade checkpoint, and filters out parsed mutations dated at/before registration. The repository skips mutation imports at/before the effective broker cutoff altogether; existing historical mutation rows are not deleted. Deposits, withdrawals, and dividends follow the mutation's `date`, not `email_received_at`, fetch time, or import time. A late email cannot make an old event affect cash. Date-only events remain midnight at the supplied date; no intraday time is invented, so events dated the registration/reset day may be excluded by that day's timestamp cutoff.
- Imported trades/mutations without a valid event date are rejected, not assigned today's date. An undated **manual** transaction still defaults to server now.
- Repeated imports do not apply cash twice. Duplicate trade timestamp enrichment is skipped if it would move an already-recorded trade across the cash cutoff.
- Manual additions and transfers adjust both stored cash and the baseline, without generating duplicate cash-mutation rows. Transfers are atomic, user-scoped, lock both brokers in ID order, require distinct brokers and a positive amount, and enforce source funds. Deleting a trade reverses its cash delta only if that trade is eligible under the current cutoff.

**Set current cash**: `PUT /broker/:id/cash-base`, authenticated, now takes only:

```json
{"cash_base": 1500000}
```

`cash_base` is the **current balance**, not a historical starting balance. Zero is supported; negative/nonfinite balances are rejected. The server sets `cash = cash_base` exactly and records the cutoff at server time while holding the broker lock (never earlier than registration). No existing events are added during the reset. A late import dated before that reset cannot change it.

The legacy `cash_base_as_of` request field remains optional and accepted, but its value is ignored/clamped to server now, whether in the past or future. Clients should omit it. The response envelope stays `{ "message": "...", "data": ... }`, including `cash`, `cash_base`, `cash_base_as_of`, `events_total`, and `events`. `GET /cash-breakdown/:id` retains the same breakdown fields and uses the same event eligibility as cash updates/recalculation. The timestamp/events are backend accounting metadata, not a date-selection requirement for Set Cash.

Existing `PUT /broker/:id` remains an **addition** of the submitted `cash`, not a current-balance replacement. Use the cash-base endpoint for replacement. The internal repository's `UpdateBrokerData` now also takes a cash delta; it must not be passed an absolute balance.

**Deployment / old data:** the forward migration `anchor_cash_to_registration` follows the existing migrations; no applied migration is rewritten. It leaves every stored `brokers.cash` unchanged, raises missing/early cutoffs to registration, retains later cutoffs, and derives `cash_base = stored cash - eligible already-stored event deltas` using event dates. This aligns breakdown/recalculation with stored cash without replaying historical money. It is intentionally not reversed by Down. Run the migration with old application writers stopped before enabling the new code; back up broker balances and baseline metadata first. Existing `NUMERIC(14,2)` baseline precision is unchanged.

**This does not repair or certify old cash balances.** Previously replayed historical trades/mutations, duplicate cash, or email-time effects may already have contaminated them. A derived legacy baseline may be negative and is not asserted to be the user's original manual balance. Users must enter their verified current broker balance through Set Cash to correct it; history/holdings need not be deleted or reimported. Brokers without a matching user/registration timestamp are not assigned invented cutoffs by the migration and need data review. Runtime fallback for missing/early baseline metadata similarly preserves stored cash, subtracting already-recorded eligible deltas to derive baseline metadata before applying new events; read-only breakdown does not persist that normalization.

Repository/migration regressions use SQLite in memory only (`gorm.io/driver/sqlite`, requiring CGO for tests); production remains PostgreSQL. No live database/Gmail is required by these tests. SQLite does not validate PostgreSQL row-lock/concurrency behavior or production numeric rounding.

### Market data (Yahoo Finance)

`GET /api/market/:ticker` requires `Authorization: Bearer <JWT>` through the existing `/api` authentication middleware. Lowercase tickers are uppercased; the resulting ticker must match `[A-Z0-9]{1,12}` without `.JK` or surrounding whitespace.

The endpoint requests Yahoo's free chart API at `/v8/finance/chart/{ticker}.JK?range=1mo&interval=1d`. Successful results are cached in memory for five minutes, with at most 256 entries per service instance; failures are never cached. No database writes or migrations are required. Yahoo data may be delayed.

**Success (`200 OK`)** — illustrative values:

```json
{
  "data": {
    "ticker": "BBCA",
    "name": "Bank Central Asia",
    "currency": "IDR",
    "exchange": "Jakarta",
    "price": 110,
    "previous_close": 100,
    "change": 10,
    "change_percent": 10,
    "day_high": null,
    "day_low": 100,
    "fifty_two_week_high": 120,
    "fifty_two_week_low": null,
    "volume": 0,
    "market_time": 1704276000,
    "fetched_at": "2026-10-08T00:00:00Z",
    "history": [{"time": 1704276000, "close": 110}]
  }
}
```

All fields are always present. Unavailable numeric fields are `null`, not zero. `name`, `currency`, and `exchange` are strings (empty if unavailable). `volume` and `market_time` are nullable integers; `market_time` and history `time` are Unix seconds. `fetched_at` is a UTC RFC 3339 / ISO 8601 timestamp and stays unchanged on cache hits. History is a chronological array of daily closes for the requested month; missing/nonfinite closes and missing timestamps are skipped, and absent history is `[]`.

`previous_close` uses Yahoo's `previousClose`, or the last valid daily close before the quote's exchange-local date when available. It does not use `chartPreviousClose`, which is the start-of-range close. `change = price - previous_close`; `change_percent = change / previous_close * 100` (null when unavailable or the previous close is zero).

**Errors:** invalid ticker `400`; missing/invalid JWT `401` via existing middleware; Yahoo not found `404`; Yahoo rate limit `429`; upstream HTTP/network errors or malformed/oversized responses `502`; upstream timeout `504`. Service errors use the existing `{"status_code": 429, "message": "Yahoo chart returned HTTP 429"}` envelope. Requests have a 10-second HTTP client timeout and successful response bodies are limited to 2 MiB.

### Transaction

| Method | Endpoint           | Description                              |
|--------|--------------------|------------------------------------------|
| `GET` | `/transaction/:id` | Get transaction by Id                    |
| `POST` | `/transaction`     | Add new transaction                      |
| `DELETE` | `/transaction/:id` | Delete broker (supports comma-separated) |


---

## Request & Response Examples

### POST `/stock`

Add a new stock holding.

**Request:**
```json
{
  "ticker": "BBRI",
  "lot": 24,
  "avg_price": 4405,
  "broker_id": 1,
  "broker_name": "Sekuritas A"
}
```

**Response:** `200 OK`
```json
{
  "message": "Stock added successfully",
  "data": [
    {
      "ticker": "BBRI",
      "lot": 24,
      "avg_price": 4405,
      "broker_id": 1,
      "broker_name": "Sekuritas A",
      "created_at": "2026-02-19T12:00:00Z",
      "updated_at": "2026-02-19T12:00:00Z"
    }
  ]
}
```

---

## Request & Response Examples

### POST `/transaction`

Add a new stock holding.

**Request:**
```json
{
   "ticker": "BTPS",
   "price": 1155,
   "lot": 5,
   "broker_id": 1
}
```

**Response:** `200 OK`
```json
{
  "status_code": 200,
  "message": "Transaction created successfully",
  "data": {
    "id": 6,
    "broker": {
      "id": 1,
      "broker_id": 0,
      "name": "Stockbit Sekuritas"
    },
    "stock": {
      "id": 3,
      "ticker": "BTPS",
      "lot": 5,
      "price": 1155
    },
    "type": "BUY",
    "date": "2026-02-26T11:10:26.078477111+08:00"
  }
}
```

---

### PUT `/stock/:ticker`

Update existing stock — adds lot and recalculates weighted average price.

**Request:**
```json
{
  "lot": 10,
  "avg_price": 3780
}
```

Hardcoded Change the lot and avarage prices.

---

### GET `/stock/:ticker`

Get stock data. Supports multiple tickers: `/stock/BBRI,ASII,PGAS`

**Response:** `200 OK`
```json
{
  "message": "Stocks retrieved successfully",
  "data": [
    {
      "ticker": "BBRI",
      "lot": 24,
      "avg_price": 4405,
      "broker_id": 1,
      "broker_name": "Sekuritas A",
      "created_at": "2026-02-19T12:00:00Z",
      "updated_at": "2026-02-19T12:00:00Z"
    }
  ]
}
```

---

### GET `/stocks/`

Get all stock data. Supports Param `/stocks?limit=2`

**Response:** `200 OK`
```json
{
  "data": [
    {
      "ticker": "BBRI",
      "lot": 24,
      "avg_price": 4312.81,
      "broker_id": 1,
      "broker_name": "Stockbit Sekuritas",
      "created_at": "2026-02-22T17:31:49.901344Z",
      "updated_at": "2026-02-22T17:31:49.901344Z"
    },
    {
      "ticker": "ACES",
      "lot": 73,
      "avg_price": 470.18,
      "broker_id": 1,
      "broker_name": "Stockbit Sekuritas",
      "created_at": "2026-02-22T17:33:10.330131Z",
      "updated_at": "2026-02-22T17:33:10.330131Z"
    }
  ],
  "message": "Success received all stocks details"
}
```


### DELETE `/stock/:ticker`

Delete stock holdings. Supports multiple: `/stock/BBRI,KETR`

---

### POST `/broker`

Add a new broker/sekuritas.

**Request:**
```json
{
  "name": "Sekuritas A",
  "cash": 500000
}
```

**Response:** `200 OK`
```json
{
  "message": "Broker added successfully",
  "data": {
    "brokers": [
      {
        "id": 1,
        "name": "Sekuritas A",
        "cash": 500000,
        "stocks_data": [],
        "stocks_count": 0
      }
    ]
  }
}
```

---

### GET `/brokers`

Get all brokers.

**Response:** `200 OK`
```json
{
  "message": "Success received all Brokers",
  "data": [
    {
      "id": 1,
      "name": "Sekuritas A",
      "cash": 500000
    }
  ]
}
```

---

### GET `/broker/:name`

Get broker details with stock holdings. Supports multiple: `/broker/Sekuritas A,Sekuritas B`

**Response:** `200 OK`
```json
{
  "message": "Broker details retrieved successfully",
  "data": {
    "brokers": [
      {
        "id": 1,
        "name": "Sekuritas A",
        "cash": 500000,
        "stocks_data": [
          {
            "ticker": "BBRI",
            "lot": 24,
            "avg_price": 4405,
            "broker_id": 1,
            "broker_name": "Sekuritas A"
          }
        ],
        "stocks_count": 1
      }
    ]
  }
}
```

---

### PUT `/broker/:id`

Update broker data (name/cash).

**Request:**
```json
{
  "name": "Sekuritas A Updated",
  "cash": 750000
}
```

---

### DELETE `/broker/:name`

Delete broker. Supports multiple: `/broker/Sekuritas A,Sekuritas B`

---

## Project Structure

```
backend/
├── main.go                      # Entry point
├── cmd/app/app.go               # App bootstrap
├── config/config.go             # Environment config
├── contract/
│   ├── repository.go            # Repository interfaces
│   └── service.go               # Service interfaces
├── dto/dto.go                   # Request/Response DTOs
├── handler/
│   ├── main.go                  # Controller registry
│   ├── error_handler.go         # Error handling
│   ├── stocks_controller.go     # Stock endpoints
│   └── brokers_controller.go    # Broker endpoints
├── internal/
│   ├── database/database.go     # DB connection
│   └── server/server.go         # HTTP server
├── migrations/                  # SQL migrations
├── models/models.go             # GORM models
├── pkg/error/error.go           # Custom errors
├── repository/
│   ├── repository.go            # Repository wiring
│   ├── stocks_repository.go     # Stock data access
│   └── brokers_repository.go    # Broker data access
├── routes/routes.go             # Route registration
└── service/
    ├── service.go               # Service wiring
    ├── stocks_service.go        # Stock business logic
    └── brokers_service.go       # Broker business logic
```

## Rate Limiting

- **100 requests per minute** per IP
- Returns `429 Too Many Requests` when exceeded

## App Version

1.0.0
