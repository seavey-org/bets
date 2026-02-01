# Bets

Friendly betting pools for groups of friends, family, and coworkers. Create groups, invite people with a code, and run betting pools using a points-based system.

## Features

- **Google OAuth** or **email/password** sign-in
- **Groups** with invite codes, configurable starting points, admin controls
- **Betting pools** with multiple options, one bet per person per pool
- **Prediction markets** with continuous trading via CPMM (prices reflect probabilities)
- **Proportional payouts** when pools are resolved
- **Points audit trail** tracking every grant, bet, win, trade, and refund
- **Leaderboard** with win/loss records per group
- **Real-time updates** via WebSockets
- **Dark/light/system theme** toggle
- **Mobile-friendly** responsive design

## Quick Start

### Prerequisites
- Go 1.24+
- Node.js 20+
- Google OAuth credentials ([console.cloud.google.com](https://console.cloud.google.com)) (optional, only needed for Google sign-in)

### Backend
```bash
cd backend
cp .env.example .env  # edit with your Google OAuth credentials
go mod tidy
go run .
```

### Frontend
```bash
cd frontend
npm install
npm run dev
```

Open `http://localhost:5173`. The Vite dev server proxies API requests to the Go backend on port 8080.

## How It Works

1. Sign in with Google or create an account with email/password
2. Create a group (you become admin, get starting points)
3. Share the invite code with friends
4. Create a betting pool with 2+ options
5. Members place bets using their points
6. Admin or pool creator resolves the pool by picking the winner
7. Winners split the pot proportionally to their wagers

### Prediction Markets

1. Create a market with a question and 2+ outcomes (e.g. "Who wins the election?" with Alice/Bob/Charlie)
2. The creator seeds initial liquidity (points deducted from their balance)
3. Buy shares of outcomes you believe in (price goes up as more people buy)
4. Sell shares to take profit or cut losses (price goes down)
5. Prices always reflect probabilities (sum to ~1.0)
6. On resolution: winning shares pay 1 point each, losing shares are worthless

## Deployment

Hosted at **bets.seavey.dev** via Docker + nginx + Cloudflare.

```bash
# Local Docker build
docker compose -f docker-compose.local.yml up --build
```

See `CLAUDE.md` for full deployment details and required environment variables.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Frontend | Vue 3, TypeScript, Vite 7, Tailwind CSS 4, Pinia 3 |
| Backend | Go 1.24, Gin, GORM, SQLite (WAL) |
| Auth | Google OAuth 2.0 + local email/password (bcrypt) + JWT |
| Real-time | WebSockets (gorilla/websocket) |
| Deploy | Docker, nginx, GitHub Actions, Cloudflare |

## CI/CD

GitHub Actions runs on every push and PR to `main`:
- **Backend:** tests (`go test -race`), formatting (`gofmt`), linting (`golangci-lint`)
- **Frontend:** linting (ESLint), type checking (`vue-tsc`), production build (`vite build`)
