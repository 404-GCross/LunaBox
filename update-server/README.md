# LunaBox Update Server

Cloudflare Worker update service backed by R2 and D1.

## Object layout

```text
channels/<channel>/version.json
releases/<version>/version.json
releases/<version>/manifest.json
releases/<version>/<asset>
```

Versioned release objects are immutable. A channel document is published last
and identifies the release whose manifest URL clients derive. `/version.json`
is an alias for the stable channel document.

## API

```text
GET  /health
GET  /version.json
GET  /v1/channels/<channel>
GET  /v1/releases/<version>/manifest
GET  /v1/releases/<version>/version
GET  /v1/releases/<version>/assets/<asset>
POST /v1/events
GET  /v1/stats/releases/<version>
GET  /v1/admin/dashboard
GET  /admin
```

The statistics and dashboard API endpoints require
`Authorization: Bearer <ADMIN_TOKEN>`. The `/admin` page asks for the token and
keeps it in the current browser tab's session storage.

The dashboard shows successful update events, updated installations, failures,
asset request volume, a 30-day update chart, per-version telemetry, and patch
source-to-target relationships read from each R2 release manifest.

## Setup

```powershell
pnpm install
Copy-Item .dev.vars.example .dev.vars
pnpm exec wrangler d1 create lunabox-updates
pnpm exec wrangler r2 bucket create lunabox-updates
pnpm exec wrangler secret put ADMIN_TOKEN --env production
pnpm exec wrangler d1 migrations apply lunabox-updates --remote --env production
pnpm deploy
```

Set a development-only token in `.dev.vars`, which is ignored by Git. Run
`pnpm types` after changing bindings or variables in `wrangler.jsonc`.
