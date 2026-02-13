# CLAUDE.md

Project-specific instructions for AI agents working on this codebase.

## Overview

Bets is a betting pools app for groups of friends, family, and coworkers. Users sign in with Google OAuth or a local email/password account, create or join groups via invite codes, and participate in betting pools using a points-based system.

**Domain:** bets.seavey.dev
**Host Port:** 3083 (container 8080)

## Tech Stack

- **Frontend:** Vue 3 + TypeScript + Vite 7 + Tailwind CSS 4 + Pinia 3
- **Backend:** Go 1.24 + Gin + GORM + SQLite (WAL mode)
- **Auth:** Google OAuth 2.0 + local email/password (bcrypt) + JWT (httpOnly cookie)
- **Real-time:** WebSockets (gorilla/websocket)
- **Deploy:** Docker multi-stage, nginx, Cloudflare CDN

See `PROJECT_STANDARDS.md` for cross-project conventions.

## Project Structure

```
backend/
├── main.go              # Entry point, route registration, SPA serving
├── config/config.go     # Env-based configuration
├── models/              # GORM models (User, Group, GroupMember, Pool, PoolOption, Bet, PointsLog,
│                        #   Market, MarketOutcome, SharePosition, Trade)
├── storage/database.go  # SQLite init with WAL mode, auto-migration, manual migrations
├── middleware/           # JWT auth middleware, group membership checks
├── handlers/            # HTTP handlers (auth, groups, pools, markets, leaderboard, websocket)
├── services/            # Business logic (auth/JWT, group mgmt, pool resolution, market CPMM, WS hub)
└── .golangci.yml        # Linter config

frontend/
├── src/
│   ├── main.ts          # App entry, Pinia + Router setup
│   ├── router.ts        # All routes with auth guards
│   ├── App.vue          # Root layout
│   ├── components/      # AppNav, GroupCard, PoolCard, MarketCard, ThemeToggle
│   ├── views/           # Login, Dashboard, GroupHome, GroupCreate, GroupJoin,
│   │                      GroupSettings, PoolCreate, PoolDetail, MarketCreate,
│   │                      MarketDetail, Leaderboard, History
│   ├── stores/          # auth, groups, pools, markets, websocket, theme
│   ├── services/api.ts  # Axios client with credential forwarding
│   └── utils/format.ts  # Date/points formatting helpers
├── vite.config.ts       # Dev proxy to backend
└── eslint.config.js     # ESLint flat config
```

## Development

### Backend
```bash
cd backend
go mod tidy
go run .
```

Required env vars (see `backend/.env.example`):
- `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` - Google OAuth credentials
- `JWT_SECRET` - Signing key for auth tokens
- `DB_PATH` - SQLite database path (default: `data/bets.db`)
- `BASE_URL` - Full URL for OAuth redirect (default: `http://localhost:8080`)

### Frontend
```bash
cd frontend
npm install
npm run dev
```

Vite dev server proxies `/api` and `/ws` to `localhost:8080`.

## Testing

```bash
cd backend && go test -race ./...
```

Tests use in-memory SQLite databases (no mocks). Each test gets a fresh DB.
Key test files:
- `services/auth_test.go` - Local registration, login, duplicate email, wrong password, Google-only account rejection, JWT round-trip
- `services/group_test.go` - Group CRUD, join, grant points, kick, invite codes, group deletion (cascade)
- `services/pool_test.go` - Full pool lifecycle: create, bet, lock, resolve (proportional split, no winners refund), cancel
- `services/market_test.go` - Prediction market lifecycle: create, buy/sell shares, CPMM price mechanics (sum-to-one, price movement), resolve payouts, cancel refunds, ClosesAt enforcement, permissions
- `handlers/auth_e2e_test.go` - E2E: registration, login, validation, /me, logout
- `handlers/groups_e2e_test.go` - E2E: group CRUD, join, grant points, kick, delete cascade
- `handlers/pools_e2e_test.go` - E2E: pool lifecycle, bet on locked, cancel, non-member 403
- `handlers/markets_e2e_test.go` - E2E: market lifecycle, buy/sell, cancel, positions, permissions

## Linting

```bash
# Backend
cd backend && golangci-lint run ./...

# Frontend
cd frontend && npx eslint .
```

Format Go code before committing:
```bash
cd backend && gofmt -w . && goimports -local github.com/codyseavey/bets -w .
```

## Key Architecture Decisions

- **Pool resolution math:** Winners split the pot proportionally to their wager. Last winner gets the remainder to avoid rounding loss. If nobody picked the winning option, all bets are refunded.
- **WebSocket auth:** Uses httpOnly cookies (sent automatically on WS upgrade handshake). The frontend does NOT need to extract or pass the token manually.
- **Points are deducted immediately** when a bet is placed and credited on resolution. All point changes are logged in `PointsLog` for a full audit trail.
- **One bet per user per pool.** Enforced by a unique composite index on `(pool_id, user_id)`.
- **Invite codes** are 8-character alphanumeric strings using an unambiguous charset (no 0/O, 1/I).
- **Local auth** uses bcrypt for password hashing. GoogleID is nullable (`*string`) so users can exist without a Google account. A manual SQLite migration in `storage/database.go` handles the NOT NULL to nullable transition on existing databases.
- **Group deletion** cascades to all related data (bets, pool options, pools, points logs, members) in a single transaction. Only group admins can delete.
- **Prediction markets** use a Constant Product Market Maker (CPMM) with complete-sets formulation. Prices always sum to ~1.0 (probabilities). Buy cost rounds up, sell payout rounds down (no value leakage). Binary markets use a closed-form quadratic; n>2 markets use binary search with dynamic bounds. A `sync.Mutex` on MarketService serializes buy/sell to prevent TOCTOU races on pool shares. The `Market.Liquidity` field stores the initial seed amount (used for cancel refunds instead of log queries).

## CI/CD

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on push/PR to main:
- **Backend job:** `go test -race`, `gofmt` check, `golangci-lint`
- **Frontend job:** ESLint, `vue-tsc --noEmit` type check, `vite build`

## Deployment

Docker multi-stage build. CI/CD via GitHub Actions deploys to `192.168.86.227`.

Required GitHub Secrets:
- `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`
- `JWT_SECRET`
- `CLOUDFLARE_ZONE_ID` / `CLOUDFLARE_API_TOKEN`

SSH deploys use the runner user's existing key in `~/.ssh`.
