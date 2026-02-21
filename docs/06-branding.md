# NightOwl — Branding & Design System

> **Product name:** NightOwl  
> **Tagline:** "The wise one that watches while you sleep."  
> **Parent brand:** A Wisbric product — [wisbric.com](https://wisbric.com)

---

## 1. Product Identity

### Name & Meaning

**NightOwl** — a 24/7 operations platform built for infrastructure teams that work across time zones. The owl is wise (knowledge base), sees in the dark (alert detection), watches through the night (on-call), and calls out when danger approaches (escalation & phone callout).

### Logo

- Primary logo: owl icon + "NightOwl" wordmark
- The owl icon should be minimal and geometric — not cartoonish
- Favicon: owl icon only, works at 16×16 and 32×32
- Use the Wisbric logo alongside NightOwl in the footer: "A Wisbric product"
- Logo files should be placed in `web/public/` as SVG (scalable) and PNG (fallback)

### Voice & Tone

NightOwl is for infrastructure engineers running critical systems. The UI copy should be:

- **Direct** — no marketing fluff in the app itself
- **Precise** — use correct technical terms (alert, incident, escalation, not "issue" or "problem")
- **Calm** — even when showing critical alerts, the UI should feel controlled, not panicked
- **Professional** — this is KRITIS-grade tooling, not a consumer app

---

## 2. Color Palette

Design inspired by the Wisbric site aesthetic: clean, professional, dark-capable. Colors chosen to work in both light and dark modes, with high contrast for on-call dashboards viewed at 3am.

### Core Colors

| Token               | Light Mode   | Dark Mode    | Usage                                    |
|----------------------|-------------|-------------|------------------------------------------|
| `--background`       | `#FFFFFF`   | `#0F1117`   | Page background                          |
| `--foreground`       | `#0F1117`   | `#E8EAED`   | Primary text                             |
| `--card`             | `#F8F9FA`   | `#1A1D27`   | Card/panel backgrounds                   |
| `--card-foreground`  | `#0F1117`   | `#E8EAED`   | Card text                                |
| `--muted`            | `#F1F3F5`   | `#252830`   | Muted backgrounds, disabled states       |
| `--muted-foreground` | `#6B7280`   | `#9CA3AF`   | Secondary text, timestamps               |
| `--border`           | `#E5E7EB`   | `#2D3039`   | Borders, dividers                        |
| `--input`            | `#E5E7EB`   | `#2D3039`   | Input field borders                      |
| `--ring`             | `#D97706`   | `#F59E0B`   | Focus rings                              |

### Brand Colors

| Token               | Value        | Usage                                    |
|----------------------|-------------|------------------------------------------|
| `--primary`          | `#1E3A5F`   | Primary actions, navigation, headers (deep navy) |
| `--primary-foreground` | `#FFFFFF` | Text on primary                          |
| `--secondary`        | `#374151`   | Secondary actions (slate gray)           |
| `--secondary-foreground` | `#FFFFFF` | Text on secondary                      |
| `--accent`           | `#D97706`   | Owl-gold accent — highlights, active states, focus |
| `--accent-foreground` | `#FFFFFF`  | Text on accent                           |

### Severity Colors

Critical for alert and incident displays. Must be instantly recognizable.

| Severity    | Color      | Token                | Usage                          |
|-------------|-----------|----------------------|--------------------------------|
| Critical    | `#DC2626` | `--severity-critical` | P1 alerts, critical incidents  |
| Warning     | `#F59E0B` | `--severity-warning`  | P2 alerts, warnings            |
| Info        | `#3B82F6` | `--severity-info`     | P3 informational alerts        |
| OK/Resolved | `#10B981` | `--severity-ok`       | Resolved, healthy, acknowledged |

### Status Colors

| Status         | Color      | Token              |
|----------------|-----------|---------------------|
| Firing         | `#DC2626` | `--status-firing`    |
| Acknowledged   | `#F59E0B` | `--status-ack`       |
| Investigating  | `#8B5CF6` | `--status-invest`    |
| Resolved       | `#10B981` | `--status-resolved`  |
| Suppressed     | `#6B7280` | `--status-suppressed`|

---

## 3. Typography

### Font Stack

```css
/* Primary — clean, modern, infrastructure-grade */
--font-sans: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;

/* Monospace — for alert labels, log entries, code blocks */
--font-mono: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace;
```

Install Inter and JetBrains Mono via Google Fonts or self-host for data sovereignty:

```html
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
```

### Type Scale

| Element          | Size   | Weight | Font       | Usage                          |
|------------------|--------|--------|------------|--------------------------------|
| Page title       | 24px   | 700    | Inter      | Dashboard title, page headers  |
| Section heading  | 18px   | 600    | Inter      | Card headers, section titles   |
| Body             | 14px   | 400    | Inter      | Default body text              |
| Body small       | 13px   | 400    | Inter      | Timestamps, metadata           |
| Caption          | 12px   | 400    | Inter      | Labels, helper text            |
| Alert label      | 13px   | 500    | JetBrains Mono | Alert names, metric keys   |
| Code/log         | 13px   | 400    | JetBrains Mono | Log lines, JSON, commands  |

---

## 4. Component Guidelines

### Built on shadcn/ui + Tailwind

The frontend uses **shadcn/ui** components with **Tailwind CSS**. All customization flows through the CSS variables above. Configure in `tailwind.config.ts`:

```typescript
// tailwind.config.ts
export default {
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent))',
          foreground: 'hsl(var(--accent-foreground))',
        },
        severity: {
          critical: 'var(--severity-critical)',
          warning: 'var(--severity-warning)',
          info: 'var(--severity-info)',
          ok: 'var(--severity-ok)',
        },
      },
      fontFamily: {
        sans: ['Inter', ...defaultTheme.fontFamily.sans],
        mono: ['JetBrains Mono', ...defaultTheme.fontFamily.mono],
      },
    },
  },
};
```

### Navigation

- **Sidebar** layout (not top nav) — standard for ops dashboards
- Sidebar: dark background (`--primary` navy), white text, accent highlight on active item
- Owl icon + "NightOwl" wordmark at top of sidebar
- Navigation groups: Dashboard, Alerts, Knowledge Base, Rosters, Escalation, Admin
- Collapse to icon-only on narrow screens
- Current tenant name displayed below the logo

### Cards & Panels

- Rounded corners: `rounded-lg` (8px)
- Subtle shadow in light mode: `shadow-sm`
- No shadow in dark mode, use border instead
- Consistent padding: `p-4` for cards, `p-6` for page sections

### Alert & Severity Badges

- Use colored dot + text label, not full-color backgrounds
- Critical: red dot, bold text
- Warning: amber dot
- Info: blue dot
- OK: green dot
- Example: `● Critical` with the dot colored, text in foreground color

### Tables (TanStack Table)

- Alternating row backgrounds in light mode
- Hover highlight row
- Sticky header
- Monospace font for alert names and metric keys
- Severity dot in first column for alert tables
- Sort indicators in column headers

### Buttons

- Primary: navy (`--primary`) background, white text, owl-gold hover ring
- Secondary: outlined, gray border
- Destructive: red background for dangerous actions (resolve, delete, escalate)
- All buttons: `rounded-md`, medium height, clear hover/active states

### Charts (Recharts)

- Use severity colors for alert charts
- Navy primary for trend lines
- Amber accent for highlights/annotations
- Grid lines: muted, thin, never distracting
- Tooltip: dark background, white text (always readable regardless of mode)

---

## 5. Layout

### Dashboard

```
┌─────────────────────────────────────────────────────┐
│ 🦉 NightOwl          [tenant: acme]     [user] [◐] │
├──────────┬──────────────────────────────────────────┤
│          │                                          │
│ Dashboard│  ┌─────────┐ ┌─────────┐ ┌─────────┐    │
│ Alerts   │  │ Active  │ │ Open    │ │ MTTR    │    │
│ KB       │  │ Alerts  │ │ Incidents│ │ (avg)   │    │
│ Rosters  │  │   12    │ │    5    │ │  42m    │    │
│ Escalate │  └─────────┘ └─────────┘ └─────────┘    │
│ Admin    │                                          │
│          │  ┌─── On-Call Now ──────────────────┐    │
│          │  │ 🇳🇿 Stefan K.  (NZ roster)       │    │
│          │  │ 🇩🇪 Max M.     (DE roster)       │    │
│          │  └─────────────────────────────────┘    │
│          │                                          │
│          │  ┌─── Alert Trend ─────────────────┐    │
│          │  │  [chart: 24h alert volume]       │    │
│          │  └─────────────────────────────────┘    │
│          │                                          │
│          │  ┌─── Top Recurring ────────────────┐    │
│          │  │  [chart: top 5 alert types]      │    │
│          │  └─────────────────────────────────┘    │
│          │                                          │
└──────────┴──────────────────────────────────────────┘
```

### Responsive

- Desktop (≥1280px): sidebar + full content area
- Tablet (768–1279px): collapsible sidebar (icon-only)
- Mobile (<768px): bottom navigation bar, full-width content

---

## 6. Dark Mode

Dark mode is the **default** — ops engineers checking alerts at 3am shouldn't be blinded.

- Toggle in header: sun/moon icon
- Persist preference in localStorage
- System preference detection as initial default
- All colors defined as CSS variables with light/dark variants
- Charts, badges, and severity indicators must remain clearly visible in both modes

---

## 7. Wisbric Integration

### Footer

Every page includes a subtle footer:

```
NightOwl v0.1.0 — A Wisbric product
```

The Wisbric logo (small, muted) links to wisbric.com.

### Login Page

- Centered card layout
- Owl icon prominently displayed
- "NightOwl" wordmark below icon
- "Sign in to continue" subtitle
- OIDC login button
- "Powered by Wisbric" at bottom

### Favicon & Meta

```html
<title>NightOwl — 24/7 Operations Platform</title>
<meta name="description" content="NightOwl — incident knowledge base, alert management, and on-call platform by Wisbric.">
<link rel="icon" type="image/svg+xml" href="/owl-icon.svg">
```

---

## 8. Implementation Notes for Claude Code

When implementing Phase 6 (Frontend):

1. **Rename all references** from "OpsWatch" to "NightOwl" throughout the codebase — API paths stay as `/api/v1/` (no product name in API routes)
2. **Install fonts** — add Inter and JetBrains Mono to the Vite/React app
3. **Configure shadcn/ui** — apply the CSS variable palette from Section 2 to `globals.css`
4. **Dark mode first** — implement with `class` strategy on `<html>`, default to dark
5. **Sidebar navigation** — use the layout structure from Section 5
6. **Severity system** — use the dot+label pattern for all alert/incident severity display
7. **Logo placeholder** — use a simple owl SVG icon (geometric, minimal) as placeholder until a final logo is provided. Generate a simple one using Lucide's `Bird` icon or similar.
8. **Tailwind config** — extend with the custom colors and fonts from Section 4
9. **Page titles** — all pages should set `document.title` to `"PageName — NightOwl"`
10. **No product name in backend** — the Go binary, Helm chart, and Docker image can remain `opswatch` or be renamed to `nightowl` — defer this decision to Stefan

---

## 9. Asset Checklist

| Asset                  | Format   | Location              | Status    |
|------------------------|----------|----------------------|-----------|
| Owl icon               | SVG      | `web/public/`        | Needed    |
| Owl icon (favicon)     | SVG+ICO  | `web/public/`        | Needed    |
| NightOwl wordmark      | SVG      | `web/public/`        | Needed    |
| Wisbric logo (small)   | SVG+PNG  | `web/public/`        | Copy from wisbric.com |
| OG image (social)      | PNG 1200×630 | `web/public/`    | Needed    |
| Inter font             | WOFF2    | `web/public/fonts/` or Google Fonts | Self-host preferred |
| JetBrains Mono font    | WOFF2    | `web/public/fonts/` or Google Fonts | Self-host preferred |
