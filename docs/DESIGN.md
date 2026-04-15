# Orca Docs Design System

A living record of the design decisions that shape the Orca documentation site. This file is intentionally **not** rendered as a VitePress page — it's excluded via `srcExclude` in `.vitepress/config.ts`. Edit it freely; it is the source of truth for "why does the site look like this?"

The design is inspired by Cursor's warm-minimalist marketing site: cream canvas, warm near-black ink, orange/crimson accents, a three-voice typographic system (gothic display, serif body, mono code), and perceptually uniform edges. Every surface leans warm — no cold whites, no cold blues, no clinical grays.

## 1. Philosophy

- **Warmth over neutrality.** The entire palette is shifted toward yellow-brown. Backgrounds are cream (`#f2f1ed`), text is warm near-black (`#26251e`), hover states are warm crimson (`#cf2d56`). Nothing is pure white or pure black.
- **Compression with breathing room.** Display type uses aggressive negative tracking. Surrounding whitespace is generous. Dense text, open layout.
- **Three typographic voices.** Display / UI (gothic), editorial (serif), code (mono). Each does one job.
- **Edges dissolve, never cut.** Borders are warm brown at low alpha (10% / 20%), not hard lines. Shadows use very large blur values for diffused atmospheric lift.
- **Light and dark share identity.** Dark mode is not a recolor — it inverts the surface scale while keeping the same brand accents, same fonts, same proportions. Ink becomes cream, cream becomes near-black, but the *feeling* is preserved.

## 2. Color Palette

### Light mode

| Role            | Token              | Value           | Use |
|-----------------|--------------------|-----------------|-----|
| Ink             | `--orca-ink`       | `#26251e`       | Primary text, headings |
| Ink 90          |                    | `rgba(38,37,30,.90)` | Body prose |
| Ink 60          |                    | `rgba(38,37,30,.60)` | Secondary text, captions |
| Ink 40          |                    | `rgba(38,37,30,.40)` | Muted text |
| Ink 20          |                    | `rgba(38,37,30,.20)` | Emphasized borders |
| Ink 10          |                    | `rgba(38,37,30,.10)` | Standard borders, dividers |
| Ink 06          |                    | `rgba(38,37,30,.06)` | Subtle fills, inline code bg |
| Surface 100     | `--orca-cream-100` | `#f7f7f4`       | Lightest surface, feature card bg |
| Surface 200     | `--orca-cream-200` | `#f2f1ed`       | Page background |
| Surface 300     | `--orca-cream-300` | `#ebeae5`       | Alt button, subtle emphasis |
| Surface 400     | `--orca-cream-400` | `#e6e5e0`       | Card backgrounds |
| Surface 500     | `--orca-cream-500` | `#e1e0db`       | Tertiary button, deeper emphasis |
| Sidebar bg      |                    | `#edebe6`       | Sidebar surface *(also used for code-block background)* |

### Dark mode

| Role            | Token                  | Value         | Use |
|-----------------|------------------------|---------------|-----|
| Ink             | `--orca-ink` (`.dark`) | `#f2f1ed`     | Primary text on dark |
| Ink 60/40/20/10/06 |                     | cream at alpha | Same role scale as light |
| Dark Surface 100 | `--orca-cream-100`     | `#1a1812`     | Card / feature bg |
| Dark Surface 200 | `--orca-cream-200`     | `#14120b`     | Page background |
| Dark Surface 300 | `--orca-cream-300`     | `#1e1c17`     | Sidebar, code block bg |
| Dark Surface 400 | `--orca-cream-400`     | `#26241e`     | Button / secondary surface |
| Dark Surface 500 | `--orca-cream-500`     | `#2c2a24`     | Tertiary button / deeper emphasis |

### Accents (shared across modes)

| Role    | Token             | Value     | Use |
|---------|-------------------|-----------|-----|
| Crimson | `--orca-error`    | `#cf2d56` | **Primary interactive accent.** Nav hover, sidebar active, "On this page" active, inline-link hover, outline marker. Also semantic error. |
| Orange  | `--orca-orange`   | `#f54e00` | Available as a token for brand moments. Not currently used for hover/active states. |
| Gold    | `--orca-gold`     | `#c08532` | Warning / highlighted contexts |
| Success | `--orca-success`  | `#1f8a65` | Tip blocks, success semantic |

> **Decision:** Originally the brand accent for hover/active was orange. We flipped `--vp-c-brand-1` to crimson (`#cf2d56`) because orange felt too urgent for dense docs. Orange is retained as a token for marketing moments.

### AI timeline colors (reserved for future use)

| Role     | Value     |
|----------|-----------|
| Thinking | `#dfa88f` (warm peach) |
| Grep     | `#9fc9a2` (sage) |
| Read     | `#9fbbe0` (blue) |
| Edit     | `#c0a8dd` (lavender) |

## 3. Typography

### Font stack

| Role        | Font                 | Fallback                  | Source        |
|-------------|----------------------|---------------------------|---------------|
| Display     | `Space Grotesk`      | Inter, system-ui, sans    | Google Fonts  |
| Sans / UI   | `Inter`              | system-ui, sans           | Google Fonts  |
| Editorial   | `Fraunces` (opsz)    | Iowan Old Style, Georgia  | Google Fonts  |
| Mono / code | `JetBrains Mono`     | ui-monospace, Menlo       | Google Fonts  |

> **Substitution rationale:** Cursor's site uses proprietary faces (CursorGothic, jjannon, berkeleyMono). We pick free Google-Fonts analogs that capture the three-voice system:
> - *Space Grotesk* → compressed gothic display feel (CursorGothic substitute)
> - *Fraunces* → opsz-enabled serif with contextual alternates (jjannon substitute)
> - *JetBrains Mono* → widely recognized dev-audience mono (berkeleyMono substitute)
>
> To upgrade to licensed faces: change the `@import` and `--orca-font-*` vars in `custom.css` — single-file swap.

### Hierarchy (doc prose)

| Role         | Font          | Size   | Weight | Line-height | Tracking   | Notes |
|--------------|---------------|--------|--------|-------------|------------|-------|
| h1           | Space Grotesk | 2.25rem | 500    | 1.15        | -0.032em   | Page title |
| h2           | Space Grotesk | 1.625rem | 500   | 1.20        | -0.02em    | Top border separator, large top margin |
| h3           | Space Grotesk | 1.25rem | 500    | 1.30        | -0.012em   | Subsection |
| Lead (h1 + p)| Fraunces      | 1.20rem | 400    | 1.50        | normal     | Editorial callout, `swsh ss01` enabled |
| Body         | Inter         | 1.00rem | 400    | 1.65        | normal     | Prose |
| Inline code  | JetBrains Mono| 0.86em | 400    | 1           | normal     | 1px border, ink-06 fill |

### Principles

- **Weight restraint.** Display type uses 500 almost exclusively. Hierarchy comes from size and tracking, not weight stacking.
- **Tracking scales with size.** The bigger the display type, the tighter the tracking.
- **Serif is for soul.** Editorial lead paragraphs break to Fraunces to give prose-heavy sections a literary warmth and signal "this is written, not emitted."
- **Header anchors hidden.** The `#` link that appears on hover is removed — it's ugly at scale and search/TOC already give anchor access.

## 4. Components

### Top navigation

- Sticky, warm-cream background with `backdrop-filter: blur(14px)` and translucent alpha.
- Links: 14px Inter, weight 500, ink at 60% → crimson on hover.
- Search pill (`Ctrl+K`): full-pill radius, single border on the `.DocSearch-Button-Keys` wrapper only (no double border).
- Top nav items: **Docs**, **Playground** (opens in new tab).

### Sidebar

- Single unified sidebar registered for `/guide/`, `/reference/`, `/examples/` — same groups everywhere.
- Groups (Cursor-style section subheadings):
  1. **Overview** — Introduction, Getting Started, CLI Reference, Syntax Overview
  2. **Examples** — Simple Agent, Multi-Agent Workflow
  3. **Language Features** — Constant Folding, Lambdas & Closures, Compile-Time Analysis
  4. **Blocks** — model, agent, tool, workflow, cron, webhook, input, schema, let
- Group headings: Space Grotesk, 13px, uppercase, `letter-spacing: 0.08em`, ink-55.
- Item links: 14px Inter, weight 500, ink-60 → crimson on hover, crimson + left indicator when active.
- **Tight vertical rhythm.** `padding: 1px 0` on `.level-1` items, `4px 10px` on `.link`, `line-height: 1.25`, `min-height: 0`. Defaults were ~80px per row — this brings it down to ~28px.
- **Light bg:** `#edebe6`. **Dark bg:** `#1e1c17`. The code-block background deliberately matches the sidebar in both modes so both surfaces read as the same system.

### Buttons

| Variant    | Light bg     | Light text | Hover bg     | Hover text | Radius | Use |
|------------|--------------|------------|--------------|------------|--------|-----|
| Primary    | `#26251e` (ink) | cream   | `#3a392f`    | cream       | 8px    | Main CTAs |
| Alt / pill | `#ebeae5` (cream-300) | ink | `#e6e5e0`  | crimson     | 8px    | Secondary actions |

- Padding: `10px 16px`, font Space Grotesk 14px weight 500.
- Hover elevates with `rgba(0,0,0,0.1) 0 4px 12px` shadow.

### Cards (feature cards)

- Background: `#f7f7f4` (cream-100)
- Border: `1px solid rgba(38,37,30,0.10)`
- Radius: 10px, padding 28px
- Hover: border → 20%, translate `-2px`, diffused shadow (`28px 70px` blur)
- Title: Space Grotesk 1.25rem weight 600, `letter-spacing: -0.015em`
- Details: Fraunces 1.02rem with `swsh ss01`, ink-60

### Code blocks

- Radius **5px**, no shadow, `1px solid ink-10` border
- Background matches sidebar: `#edebe6` (light) / `#1e1c17` (dark)
- Font: JetBrains Mono 13px, line-height 1.7
- Padding: `20px 24px`
- **Line numbers enabled** globally (`markdown.lineNumbers: true`)
  - Gutter: 32px with 6px right padding, text-align right
  - Code padding-left: 8px (tight gap between numbers and code)
- **Copy button:** 28×28 with 14px icon, positioned 10px from top-right. Only dimensions and position are overridden — background image/color are left to VitePress defaults so the icon always renders.
- Inline code: `rgba(38,37,30,0.06)` fill, `1px solid ink-10`, radius 4px, `0.86em`

### Custom containers (tip / warning / danger / info)

- Radius 10px, `1px solid` in the matching semantic color at ~25% alpha
- Tip: success green wash
- Warning: gold wash
- Danger: crimson wash
- Info: neutral ink-06 wash

### Hero (home page)

- Display name: Space Grotesk 500, `clamp(2.5rem, 6vw, 4.5rem)`, tracking `-0.035em`
- Subhead: Space Grotesk 500, `clamp(1.75rem, 4vw, 2.75rem)`, tracking `-0.022em`, ink-60
- Tagline: Fraunces 1.2rem, max-width 560px, ink-60, `swsh ss01`
- No radial glow behind the logo (explicitly disabled via `--vp-home-hero-image-background-image: none`)
- Logo: `logo.png` (black orca) in light mode, `logo-dark.png` (white orca) in dark mode
- Action buttons: Get Started (primary) / Language Reference (alt) / Playground (alt, opens new tab)

### Doc footer

- **Prev / Next page links are disabled globally** via `docFooter: { prev: false, next: false }`.

## 5. Layout & Spacing

- Max content width: 1440px
- Base unit: 8px; sub-8px scale (1.5, 2, 2.5, 3, 4, 5, 6) for micro-alignment
- Section separation via background tone shifts and spacing, not hard dividers
- h2 has a top border (`1px solid ink-10`) and `56px` top margin — the only in-prose rule

## 6. Interaction & Motion

| Trigger     | Treatment |
|-------------|-----------|
| Link hover  | Color → crimson, underline color → crimson (150ms) |
| Button hover| Text → crimson, `0 4px 12px` shadow (200ms) |
| Card hover  | Border 10% → 20%, `translateY(-2px)`, diffused shadow (220ms) |
| Sidebar hover | Text → crimson |
| Sidebar active | Text + 2px left indicator → crimson |

## 7. Chrome & affordances

- **Scrollbars hidden** (`scrollbar-width: none`, `::-webkit-scrollbar { display: none }`). Scrolling still works via wheel/touch/keyboard — the indicator itself was ugly on warm surfaces.
- **Header anchors hidden** on hover. The `#` marker is removed; cursor stays as default on headings.
- **Selection**: `rgba(245,78,0,0.18)` warm orange wash on ink text.

## 8. Playground link behavior

The Playground is a separate Next.js app:

- **Dev** (`pnpm run dev`) → `http://localhost:3000` (studio runs on its own port)
- **Prod** (`vitepress build` for GitHub Pages) → `/studio/` which resolves to `/orca/studio/` under the base
- Both the nav link and hero action use a `__PLAYGROUND_URL__` placeholder that `transformPageData` in `config.ts` rewrites at build time based on `NODE_ENV`.
- Always opens in a new tab (`target: _blank`).

## 9. Implementation files

| File | Role |
|------|------|
| `.vitepress/config.ts` | Nav, sidebar groups, markdown settings, playground link logic, docFooter disabled, light/dark toggle enabled |
| `.vitepress/theme/custom.css` | All palette tokens, VitePress variable overrides, component styling, light/dark modes |
| `.vitepress/theme/index.ts` | Imports `custom.css` on top of DefaultTheme |
| `index.md` | Home hero + feature cards |
| `reference/features/*.md` | Language feature pages (constant folding, closures, analyzer) |

## 10. Design tokens not currently used

These are declared in `custom.css` for future use but not yet applied anywhere in the UI:

- AI timeline colors (thinking / grep / read / edit) — intended for a future "how Orca compiles" timeline visualization on the marketing pages
- Orange accent (`--orca-orange`) — kept as a token in case a marketing moment needs the higher-urgency accent
- Gold (`--orca-gold`) — warning variant

---

## Change log

| Date (rel.) | Change |
|-------------|--------|
| Initial rewrite | Full Cursor-style warm palette, three-voice typography, component overrides |
| +1 | Dark mode re-enabled with inverted warm palette; hero glow removed; code block bg switched from dark slab to warm-light so Shiki light-theme tokens stay legible |
| +2 | Logo light/dark mapping corrected (light → black orca, dark → white orca) |
| +3 | Playground link env-aware (`localhost:3000` in dev, `/studio/` in prod) |
| +4 | Doc footer prev/next disabled; sidebar vertical rhythm tightened |
| +5 | Sidebar grouped into Overview / Examples / Language Features / Blocks; unified across `/guide/`, `/reference/`, `/examples/`; top nav reduced to `Docs` / `Playground` |
| +6 | Page/sidebar dark colors set to `#14120b` / `#1e1c17`; header anchors hidden |
| +7 | Scrollbars hidden globally |
| +8 | Brand accent flipped from orange to crimson for all interactive states |
| +9 | Ctrl+K wrapper border unified to a single outline |
| +10 | Playground opens in new tab |
| +11 | Light sidebar → `#edebe6`; code block bg matches sidebar in both modes |
| +12 | Code block radius → 5px, shadow removed |
| +13 | Line numbers enabled; gutter tightened |
| +14 | Copy button shrunk to 28×28 with 14px icon |
