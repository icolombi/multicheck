# Deferred upgrades

Work the dependency-audit skill found but did not apply. The skill reads this
file in Phase 0, **re-verifies every item against current data** (versions,
EOL dates and peer ranges below are a snapshot), and deletes an entry once it
has been applied or is no longer relevant.

Snapshot date: 2026-09-26 (project at v1.6.3). All npm trials were run in
throwaway copies of `frontend/`: `npm run check`, `lint` and `build` only. Dev
server and runtime behaviour were not exercised.

## Major upgrades (frontend)

| Priority | Upgrade | Trial result | What applying it takes |
|----------|---------|--------------|------------------------|
| **High** | `eslint` 9.39.5 → 10.x, `@eslint/js` 9 → 10, `globals` 15 → 17 | check/build green, lint red: 2 new `preserve-caught-error` errors | ESLint 9 is **no longer supported** (`npm warn deprecated` in CI). Attach `{ cause: error }` to the two `throw new Error(...)` in the `catch` block of `frontend/src/lib/api.ts` (~lines 36 and 38). `typescript-eslint` 8.70+ and `eslint-plugin-svelte` 3.23+ already accept ESLint 10. Needs Node ^20.19 / ^22.13 / >=24 |
| Medium | `vite` 7 → 8 together with `@sveltejs/vite-plugin-svelte` 6 → 7 | check/lint/build green **only with a regenerated lockfile** | `npm install` fails with ERESOLVE against the current lockfile (stale peer edge via `vite-plugin-svelte-inspector`); delete `package-lock.json` and reinstall, then review the lockfile diff. Vite 8 is Rolldown-based: try `npm run dev` and `npm run preview`, not only the build. Must move as a pair; `@sveltejs/kit` and `@tailwindcss/vite` already accept Vite 8 |
| Low | `prettier-plugin-svelte` 3 → 4 | check/build green, lint red until reformatted | Run prettier on `frontend/src/lib/components/CheckForm.svelte` (it moves the closing `</textarea>` of two textareas, ~lines 235 and 249). Needs Node >= 20 |
| Low | `svelte-sonner` 0.3.28 → 1.x | check/lint/build green | Toasts (`Toaster` in `+layout.svelte`, `toast` in `CheckForm`/`ResultsCard`) must be checked visually, including dark mode |
| Low | `typescript` 5.9 → 6.0.x (`~6.0.3`) | check/lint/build green | Do **not** go to 7: `@sveltejs/kit` peers stop at TypeScript 6 and `typescript-eslint` requires `<6.1.0`. Re-check both peer ranges before moving |

Suggested order: ESLint 10 first (unsupported line), then the Vite pair, then
the low-risk ones in one go.

## Revisit on a date

- **Node 26** enters LTS on 2026-10-28 (endoflife.date). After that date it
  becomes a valid target to replace Node 24 in `frontend/Dockerfile`,
  `docker-compose.yml` (frontend-dev) and the CI `node-version`; Node 24 itself
  stays supported until 2028-04, so there is no urgency.

## Not done in the last run

- **`govulncheck` was skipped** by choice, so the Go modules have never been
  scanned for known vulnerabilities by this skill. Ask again next run.
- Indirect Go modules with newer versions (`testify`, `go-cmp`, `go-internal`,
  `golang.org/x/tools`) were left alone: they only appear in the viper test
  graph, not in the compiled binary.
- `github.com/gorilla/mux` (last release v1.8.1, 2023-10) and
  `github.com/dchest/validator` (single maintainer) are maintenance-risk
  notes, not findings; no replacement was proposed.
