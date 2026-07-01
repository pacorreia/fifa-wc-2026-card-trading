# FIFA WC 2026 Card Trading

Production-ready MVP for a self-hosted World Cup 2026 Panini sticker trading app with Go, PostgreSQL, WebSockets, Docker, and Helm.

## Features

- JWT authentication with access and refresh tokens
- PostgreSQL-backed collections, trades, and notifications
- Real-time WebSocket notifications
- Missing sticker, duplicate, and trade-match dashboards
- Docker Compose for local development
- Hardened Helm chart with ingress, HPA, PDB, and NetworkPolicy

## Architecture Overview

- **Backend:** Go 1.22 REST API + Gorilla WebSocket hub
- **Frontend:** Vanilla HTML/CSS/JS SPA
- **Database:** PostgreSQL with SQL migrations and sticker seed data
- **Deployment:** Dockerfiles + controller-agnostic Helm chart

Project layout:

- `cmd/api`: application entrypoint
- `internal/auth`: JWT, bcrypt, auth middleware
- `internal/config`: environment configuration
- `internal/db`: database connection and migrations
- `internal/models`: shared models
- `internal/handlers`: HTTP and WebSocket handlers
- `internal/services`: business logic
- `internal/ws`: WebSocket hub and event helpers
- `frontend/`: SPA assets
- `migrations/`: schema and seed data
- `charts/fifa-wc-2026-card-trading/`: Helm chart

## Local Development

### Start with Docker Compose

```bash
docker compose up --build
```

Services:

- Frontend: `http://localhost:8081`
- Backend API: `http://localhost:8080`
- PostgreSQL: `localhost:5432`

### Run backend locally

```bash
go mod tidy
go run ./cmd/api
```

## Environment Variables

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | Backend listen port |
| `DATABASE_URL` | constructed from DB_* vars | PostgreSQL connection string |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL username |
| `DB_PASSWORD` | `postgres` | PostgreSQL password |
| `DB_NAME` | `fifa_wc_2026` | PostgreSQL database |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `JWT_SECRET` | `change-me-in-production` | JWT signing secret |
| `JWT_ACCESS_TTL` | `15m` | Access token TTL |
| `JWT_REFRESH_TTL` | `168h` | Refresh token TTL |
| `CORS_ORIGINS` | `http://localhost:3000` | Comma-separated allowed origins |
| `API_RATE_LIMIT_PER_MINUTE` | `100` | Authenticated API limit per user |
| `LOGIN_RATE_LIMIT_PER_MINUTE` | `5` | Login limit per IP |
| `REGISTER_RATE_LIMIT_PER_MINUTE` | `3` | Register limit per IP |
| `WS_MAX_CONNECTIONS_PER_USER` | `5` | Max concurrent WS connections per user |
| `LOG_LEVEL` | `INFO` | Structured log level |
| `SHUTDOWN_TIMEOUT` | `15s` | Graceful shutdown timeout |

## Helm Install Examples

### NGINX ingress

```bash
helm upgrade --install fifa charts/fifa-wc-2026-card-trading   -f charts/fifa-wc-2026-card-trading/values-nginx.yaml   --set secrets.databaseUrl='postgres://postgres:postgres@postgres:5432/fifa_wc_2026?sslmode=disable'   --set secrets.jwtSecret='replace-me'
```

### Traefik ingress

```bash
helm upgrade --install fifa charts/fifa-wc-2026-card-trading   -f charts/fifa-wc-2026-card-trading/values-traefik.yaml   --set secrets.databaseUrl='postgres://postgres:postgres@postgres:5432/fifa_wc_2026?sslmode=disable'   --set secrets.jwtSecret='replace-me'
```

### Existing secret pattern

Create a secret containing `DATABASE_URL` and `JWT_SECRET`, then install with:

```bash
helm upgrade --install fifa charts/fifa-wc-2026-card-trading   --set secrets.existingSecret=fifa-app-secrets
```

## Security Notes

- Passwords are hashed with bcrypt
- JWT access and refresh tokens are separated
- Refresh tokens are stored hashed server-side
- Parameterized SQL is used throughout
- Resource ownership is enforced for collections, trades, and notifications
- WebSocket authentication is required at handshake time
- Rate limiting is applied to login, register, and authenticated API traffic
- CORS is configurable
- Helm defaults harden containers with non-root users, dropped capabilities, and `RuntimeDefault` seccomp

## Known Limitations

- Trade acceptance updates trade state but does not automatically transfer sticker ownership
- WebSocket fanout is in-memory only; Redis or NATS would be needed for multi-replica event distribution
- Seed data is a representative subset rather than the full ~650-sticker album
- Frontend is intentionally lightweight and does not include server-side rendering or offline support
