# Deployment Guide

## Prerequisites

- Docker and Docker Compose
- PostgreSQL 15+ database (or Supabase account)
- Domain with SSL certificate (for production)
- Mailgun account (for email authentication)

## 1. Database Setup

Create a PostgreSQL database and run the schema:

```bash
psql -U postgres -d your_database -f schema.sql
```

Or use **Supabase** (recommended for managed hosting):
1. Create a project at https://supabase.com
2. Go to SQL Editor and paste contents of `schema.sql`
3. Copy the connection string from Settings > Database

## 2. Environment Configuration

```bash
cp .env.example .env
```

Edit `.env` with your values:

### Required

```env
# Server
PORT=8081
EXTERNAL_HOST=https://your-domain.com
DATABASE_URL=postgres://user:password@host:5432/database

# Authentication (Mailgun - required for user registration)
MAILGUN_DOMAIN=mail.your-domain.com
MAILGUN_API_KEY=your-mailgun-api-key
MAILGUN_FROM_EMAIL=noreply@your-domain.com
MAILGUN_FROM_NAME=Khabaroff Studio: Obsidian Webhooks
MAGIC_LINK_BASE_URL=https://your-domain.com
```

### Optional

```env
# JWT secret (auto-generated if not set)
JWT_SECRET=your-32-char-minimum-secret

# Event encryption at rest (AES-256-GCM)
# Generate: openssl rand -hex 32
ENCRYPTION_KEY=your-64-char-hex-string

# Event retention (default: 30 days)
EVENT_TTL_DAYS=30
ENABLE_AUTO_CLEANUP=true

# Magic link expiry (default: 3600 seconds = 1 hour)
MAGIC_LINK_EXPIRY=3600

# Marketing automation (optional - server works without it)
MAILERLITE_API_KEY=your-mailerlite-key
MAILERLITE_GROUP_SIGNUPS=group-id
MAILERLITE_GROUP_ACTIVE=group-id

# Admin user auto-seed (created on first run if no admins exist)
ADMIN_USERNAME=admin
ADMIN_PASSWORD=your-secure-password-min-8-chars

# Product analytics (optional - disabled by default)
POSTHOG_API_KEY=your-posthog-key
POSTHOG_HOST=https://eu.i.posthog.com
POSTHOG_ENABLED=false
```

### What's Optional

The server starts and works fully without these services:

| Service | Without it | Impact |
|---------|-----------|--------|
| MailerLite | Server works normally | No marketing emails |
| PostHog | Server works normally | No analytics tracking |
| Encryption key | Server works normally | Event data stored in plaintext |
| Mailgun | Server starts, but auth disabled | Users can't register/login |

## 3. Deploy with Docker

```bash
# Build and start
docker compose up -d

# Check health
curl http://localhost:8081/health

# View logs
docker compose logs -f
```

The container runs with `network_mode: host`, binding directly to the configured PORT (default: 8081).

The server will:
- Start on the configured PORT (default: 8081)
- Connect to PostgreSQL
- Auto-generate JWT secret if not provided
- Auto-clean expired events every hour (if enabled)
- Log startup configuration and service status

## 4. Reverse Proxy (Nginx)

See `nginx.conf` for the full reference configuration. Key points:

```nginx
upstream webhooks_server {
    server localhost:8081;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    client_max_body_size 10M;

    location / {
        proxy_pass http://webhooks_server;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_read_timeout 3600s;
        proxy_buffering off;
        proxy_cache off;
    }

    # Dedicated SSE location (critical for real-time delivery)
    location /events {
        proxy_pass http://webhooks_server;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_read_timeout 3600s;
        proxy_buffering off;
        proxy_cache off;
        proxy_redirect off;

        default_type text/event-stream;
    }
}

server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$host$request_uri;
}
```

Key settings for SSE:
- `proxy_buffering off` - prevents Nginx from buffering SSE events
- `proxy_read_timeout 3600s` - keeps SSE connections alive
- Separate `/events` location with `Connection ""` header for proper SSE streaming
- `proxy_http_version 1.1` - HTTP/1.1 required for SSE

## 5. Create Admin User

Set `ADMIN_USERNAME` and `ADMIN_PASSWORD` in your `.env` file. The admin user is created automatically on first startup if no admins exist in the database.

```env
ADMIN_USERNAME=admin
ADMIN_PASSWORD=your-secure-password
```

After the first run, you can remove these variables from `.env` — the admin account persists in the database.

## 6. Verify Deployment

```bash
# Health check
curl https://your-domain.com/health

# Server info
curl https://your-domain.com/info

# Landing page
open https://your-domain.com
```

## 7. Monitoring

### Health Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Database connectivity, server status |
| `GET /info` | Server version and configuration |

### Docker Health Check

Docker Compose includes automatic health monitoring:
- Checks `/health` every 30 seconds
- 3 retries with 10s timeout before marking unhealthy
- Restarts on failure (`restart: unless-stopped`)

### Log Monitoring

```bash
# Follow logs
docker compose logs -f

# Check for errors
docker compose logs --since 1h | grep -i error
```

## 8. CI/CD

GitHub Actions pipeline (`.github/workflows/ci.yml`):

```
Push to main → Run Go tests (with PostgreSQL) → Deploy to production (SSH + Docker)
```

Deployment is automatic on push to `main`.

## Updating

```bash
git pull origin main
docker compose build --no-cache
docker compose up -d
```

## Frontend CSS

Tailwind CSS is pre-built and committed to the repository. Docker does not require Node.js.

If you modify HTML templates and need to rebuild CSS:

```bash
npm install        # first time only
make css           # rebuild src/templates/assets/tailwind.css
```

## Troubleshooting

### Server won't start
- Check `docker compose logs` for error messages
- Verify `DATABASE_URL` is correct and accessible
- Ensure PostgreSQL allows connections (check firewall, IPv6 if using Supabase)

### Users can't register
- Verify `MAILGUN_API_KEY` and `MAILGUN_DOMAIN` are set
- Check Mailgun dashboard for delivery status
- Verify `MAGIC_LINK_BASE_URL` matches your actual domain

### SSE not working through proxy
- Ensure `proxy_buffering off` in Nginx config
- Verify separate `/events` location exists with `Connection ""` header
- Check that no upstream proxy caches responses
- Verify `Content-Type: text/event-stream` headers pass through

### Database connection fails with Supabase
- Supabase requires IPv6 - `network_mode: host` is already set in docker-compose.yml
- Verify the connection string includes `?sslmode=require` if needed
