# 021: Plugin Directory Submit Preparation

**Status:** COMPLETED
**Date:** 2026-02-26
**Scope:** plugin/

## Summary

Prepare the Webhooks V2 plugin for submission to the Obsidian Community Plugin Directory by meeting all requirements and guidelines.

## Changes

### Removed
- PostHog analytics (`analytics.ts`, posthog-js dependency)
- `console.log` in onload/onunload (only debug-gated logging remains)
- Inline `<style>` block from settings_tab.ts
- Emojis from settings UI
- `fundingUrl` from manifest.json
- Statistics section from settings (removed per user edit)

### Changed
- Plugin ID: `obsidian-webhooks` -> `webhooks-v2` (no "obsidian" in ID — auto-reject rule)
- Plugin name: `Obsidian Webhooks` -> `Webhooks V2`
- Author: `trashhalo` -> `khabaroff` (in package.json)
- Description: starts with verb, ends with period, <250 chars
- Settings headings: sentence case, `Setting().setHeading()` instead of `createEl("h4")`
- Status indicators: removed emojis, using CSS classes only
- Styles: extracted to `styles.css` with Obsidian CSS variables for dark theme support
- Settings tab: removed redundant `h2` header (Obsidian shows plugin name automatically)
- file_handler.ts: added `normalizePath()` on all user-defined paths (required by guidelines)

### Added
- `styles.css` — plugin styles using Obsidian CSS variables
- `versions.json` — version-to-minAppVersion mapping
- `version-bump.mjs` — standard version bump script
- `LICENSE` — MIT
- `README.md` — features, requirements, installation, usage, network disclosure, credits
- `.github/workflows/release.yml` — automated release on tag push (bun build + ncipollo/release-action)

## Compliance Checklist
- [x] No "obsidian" in plugin ID
- [x] No innerHTML/outerHTML/insertAdjacentHTML
- [x] No window.app (uses this.app)
- [x] No var (const/let only)
- [x] No .then() chains (async/await)
- [x] No client-side telemetry
- [x] No inline styles (CSS in styles.css)
- [x] No HTML heading elements in settings (uses setHeading())
- [x] Sentence case in UI text
- [x] normalizePath() on user-defined paths
- [x] console.log gated behind debug setting
- [x] console.error only for actual errors
- [x] Network usage disclosed in README
- [x] Credits to @trashhalo (original author)
- [x] manifest.json description <250 chars, starts with verb, ends with period
- [x] Release artifacts: main.js, manifest.json, styles.css

## Decisions
- Plugin ID = `webhooks-v2` (compliant, distinct from original `obsidian-webhooks`)
- GitHub repo name = `obsidian-webhooks-v2` (repo name CAN contain "obsidian")
- PostHog removed entirely from plugin (server-side analytics remain unchanged)
- `fundingUrl` removed (not needed for initial submission)
- Build artifact `main.js` = 17KB (clean, no analytics bloat)
- Credits to @trashhalo in README
- Structure follows trashhalo pattern: monorepo, plugin distributed via GitHub releases

## Next Steps (Manual)
1. GitHub repo `khabaroff-studio/obsidian-webhooks-v2` already created (empty)
2. Copy `plugin/` contents to new repo root
3. Push with tag `2.0.1` -> automated release
4. PR to `obsidianmd/obsidian-releases` -> `community-plugins.json`
