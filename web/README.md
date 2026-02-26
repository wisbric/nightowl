# NightOwl Web

The NightOwl frontend — a React 19 SPA for incident response, alert triage, and on-call operations.

## Tech Stack

- **React 19** + TypeScript 5.9
- **Vite 7** (dev server + build)
- **Tailwind CSS 4**
- **TanStack Router** + **TanStack Query**
- **lucide-react** icons

## Development

```bash
# From repo root — start backend + dependencies first
docker compose up -d
make migrate
make seed-demo
make api      # API on :8080

# Start the frontend dev server
cd web
npm install
npm run dev   # http://localhost:3000
```

The Vite dev server proxies `/api`, `/auth`, and `/status` requests to `http://localhost:8080`.

## Build

```bash
npm run build   # TypeScript check + Vite production build → dist/
npm run preview # Preview the production build locally
```

## Project Structure

```
src/
├── components/         # UI components and layouts
├── contexts/           # Auth/session context
├── hooks/              # Custom hooks
├── lib/                # API client and utilities
├── pages/              # Route pages
├── styles/             # Global styles and tokens
├── main.tsx            # App entry + router
└── index.css           # Theme tokens
```
