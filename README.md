# Obsidian Webhooks Selfhosted

Self-hosted webhook server for Obsidian with exactly-once delivery guarantee. Turn any external event (emails, Slack messages, API calls) into notes in your Obsidian vault automatically.

---

## Features

- **Real-time Delivery:** Server-Sent Events (SSE) for instant webhook notifications
- **Offline Sync:** Polling fallback ensures events arrive even when offline
- **Exactly-Once Delivery:** ACK system guarantees each event is processed exactly once
- **Self-Hosted:** Full control over your data - no third-party services required
- **Passwordless Auth:** Email magic links - no passwords to remember
- **Encryption at Rest:** AES-256-GCM encryption for webhook event data
- **Multi-Source:** Works with Gmail, Slack, Zapier, IFTTT, Make, custom webhooks
- **Production-Ready:** PostgreSQL backend, rate limiting, auto-cleanup, health checks, CI/CD

---

## Quick Start

### 1. Deploy Server

```bash
git clone https://github.com/khabaroff/obsidian-webhooks-selfhosted.git
cd obsidian-webhooks-selfhosted

cp .env.example .env
# Edit .env with your DATABASE_URL, JWT_SECRET, MAILGUN credentials

docker compose up -d

curl http://localhost:8081/health
# {"status":"healthy","database":"connected"}
```

### 2. Register

1. Open `http://localhost:8081` in your browser
2. Enter your email to receive a magic link
3. Click the link in your email to activate your account
4. Your webhook key and client key are created automatically

### 3. Install Obsidian Plugin

1. Download plugin from your dashboard or build from source:
   ```bash
   cd plugin && bun install && bun run build
   ```
2. Copy `main.js`, `manifest.json` to `.obsidian/plugins/obsidian-webhooks/`
3. Enable plugin in Obsidian Settings > Community Plugins
4. Enter your **Client Key** and **Server URL** in plugin settings

### 4. Send Your First Webhook

```bash
WEBHOOK_KEY="wh_your_key_here"
SERVER="http://localhost:8081"

curl -X POST "$SERVER/webhook/$WEBHOOK_KEY?path=inbox/test.md" \
  -H "Content-Type: application/json" \
  -d '{"title": "Hello from webhook!", "message": "It works!"}'

# Check Obsidian vault - inbox/test.md should appear
```

---

## Architecture

```
External Services (Zapier, IFTTT, Make, custom)
        │ HTTP POST
        ▼
┌──────────────────┐
│  Go Server (Gin) │ ── Rate Limiting, Validation, Encryption
│  /webhook/{key}  │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│   PostgreSQL     │ ── Events stored encrypted (AES-256-GCM)
│   (Supabase)     │ ── 30-day TTL, auto-cleanup
└────────┬─────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌────────┐ ┌────────┐
│  SSE   │ │ Poll   │ ── Real-time or fallback delivery
└───┬────┘ └───┬────┘
    └────┬─────┘
         ▼
┌──────────────────┐
│ Obsidian Plugin  │ ── File creation, dedup, ACK
└──────────────────┘
```

---

## API

| Method | Path | Description |
|--------|------|-------------|
| POST | `/webhook/{webhook_key}?path=file.md` | Send webhook event |
| GET | `/events/{client_key}` | SSE event stream |
| POST | `/ack/{client_key}/{event_id}` | Acknowledge event |
| POST | `/auth/register` | Register (sends magic link) |
| GET | `/auth/verify?token=xxx` | Verify magic link |
| GET | `/dashboard` | User dashboard |
| GET | `/admin` | Admin panel (server operator) |
| GET | `/health` | Health check |

### Webhook Request Format

```
POST /webhook/{webhook_key}?path=inbox/note.md
Content-Type: application/json
```

**Query parameters:**

| Param | Required | Description |
|-------|----------|-------------|
| `path` | Yes | File path in vault, e.g. `inbox/note.md`. Max 512 chars. |

**Request body** — the server stores any body as raw bytes (max 10 MB). The Obsidian plugin parses the body and converts JSON to a Markdown note with YAML frontmatter:

| JSON Field | Type | Description |
|------------|------|-------------|
| `content` | string | Note body in Markdown. Becomes the file content below frontmatter. |
| `title` | string | Note title — goes to frontmatter `title:` |
| `tags` | string[] | Tags array — goes to frontmatter `tags:` |
| `date` | string | Date — goes to frontmatter `date:` |
| `*` | any | Any other field goes to frontmatter as-is |

**JSON body example** — what you send vs. what appears in vault:

```json
// You send:
{
  "title": "Meeting Notes",
  "tags": ["work", "meeting"],
  "source": "n8n",
  "content": "# Standup\n\n- Done: feature X\n- Next: feature Y"
}
```

```markdown
// File in vault (inbox/note.md):
---
title: Meeting Notes
tags: ["work", "meeting"]
source: n8n
---

# Standup

- Done: feature X
- Next: feature Y
```

**Plain text body** (non-JSON) is written to the file as-is, without frontmatter.

**Response codes:**

| Code | Description |
|------|-------------|
| `200 OK` | Event accepted and queued |
| `400 Bad Request` | Missing or invalid `path` parameter |
| `401 Unauthorized` | Invalid webhook key |
| `413 Too Large` | Payload exceeds 10 MB |

---

## Use Cases

**Email to Notes (via Zapier/Make):**
```
POST /webhook/wh_xxx?path=inbox/emails/{{date}}.md
Body: {"title": "{{subject}}", "content": "From: {{sender}}\n\n{{body}}"}
```

**Slack to Tasks:**
```
POST /webhook/wh_xxx?path=projects/tasks.md
Body: - [ ] {{message}} (from @{{user}})
```

**GitHub Issues:**
```
POST /webhook/wh_xxx?path=projects/github-issues.md
Body: {"title": "{{issue.title}}", "content": "{{issue.body}}", "tags": ["github"]}
```

**Custom API:**
```python
import requests
requests.post(
    "https://your-server/webhook/wh_xxx?path=logs/api.md",
    json={"content": f"API result at {datetime.now()}: {result}", "source": "api"}
)
```

---

## Admin Panel (Self-Hosted)

When self-hosting, you need an admin account to manage users and monitor the server. The admin panel (`/admin`) provides:

- **User management** — view all registered users and their keys
- **Key activation/deactivation** — enable or revoke user access
- **Monitoring** — view undelivered events and server health

### Creating an Admin User

Add credentials to your `.env` file before the first launch:

```env
ADMIN_USERNAME=admin
ADMIN_PASSWORD=your-secure-password
```

On first start, the server checks if any admin users exist. If not, it auto-creates one from these env vars. After the admin is created, you can remove the password from `.env` — it won't be needed again.

Admin auth uses JWT tokens with a 24-hour expiry, stored in an HTTP-only cookie. Log in at `/admin`.

> **Note:** Regular users register themselves via magic links at `/login`. The admin panel is for the server operator, not end users.

---

## Configuration

### Required Environment Variables

```env
DATABASE_URL=postgres://user:pass@host:5432/db
PORT=8081
JWT_SECRET=your-random-secret-key
ENCRYPTION_KEY=64-char-hex-string
```

### Email (Mailgun)

```env
MAILGUN_DOMAIN=mail.yourdomain.com
MAILGUN_API_KEY=your-mailgun-key
MAILGUN_FROM_EMAIL=postmaster@mail.yourdomain.com
```

### Admin (first run)

```env
ADMIN_USERNAME=admin
ADMIN_PASSWORD=your-secure-password
```

### Optional

```env
EVENT_TTL_DAYS=30
ENABLE_AUTO_CLEANUP=true
POSTHOG_ENABLED=false
MAILERLITE_API_KEY=your-key
```

---

## Development

```bash
# Prerequisites: Go 1.24+, Docker, PostgreSQL

# Run tests (72 tests)
make test

# Run with race detector
go test -race ./...

# Lint
make lint

# Start test database
docker compose -f docker-compose.test.yml up -d

# Build
make build
```

### Project Structure

```
main.go                    # Server entry point & routes
schema.sql                 # Database schema
src/
├── handlers/              # HTTP handlers (webhook, SSE, ACK, auth, admin)
├── services/              # Business logic (keys, events, auth, email, analytics)
├── middleware/             # Auth, rate limiting, validation, logging
├── models/                # Data models & constants
├── database/              # Connection pool & test helpers
├── repositories/          # Interfaces & mocks
└── templates/             # HTML pages & email templates
plugin/                    # Obsidian plugin (TypeScript)
```

---

## Security

- **Passwordless auth** with crypto-secure magic links (32 bytes, one-time use, 60-min expiry)
- **AES-256-GCM** encryption for event data at rest
- **Rate limiting** per IP (auth: 3/min) and per webhook key
- **CORS whitelist** (Obsidian app, production domain only)
- **JWT sessions** with crypto/rand secret generation
- **Request body limits** via io.LimitReader
- **Mutex protection** for SSE client map

---

## License

MIT License - see LICENSE file for details.

---

**Made for the Obsidian community. Own your data. Self-hosted. Open source.**
