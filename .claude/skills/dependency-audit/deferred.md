# Deferred upgrades

Work the dependency-audit skill found but did not apply. The skill reads this
file in Phase 0, **re-verifies every item against current data** (versions,
EOL dates and peer ranges below are a snapshot), and deletes an entry once it
has been applied or is no longer relevant.

Snapshot date: 2026-10-01 (project at v1.6.5). All npm trials were run in
throwaway copies of `frontend/`: `npm run check`, `lint` and `build` only. Dev
server and runtime behaviour were not exercised.

## Major upgrades (frontend)

- **SvelteKit 3 family** (`@sveltejs/kit` 2.70.3 -> 3.0.0, `adapter-static`
  3.0.10 -> 4.0.0, `adapter-auto` 7.0.1 -> 8.0.0). Postponed by the user on
  2026-10-01. Peers are already satisfied (Vite `^8.0.12`, Svelte `^5.57.1`,
  plugin-svelte `^7`, TypeScript `^6`, Node `>=22.17`). The trial passed
  `check`, `lint` and `build` with 0 errors after these forced changes:
  1. Delete `svelte.config.js`; pass `preprocess` and `adapter` (no `kit`
     namespace) to `sveltekit({...})` in `vite.config.ts`.
  2. Drop the `svelteConfig` import/option from `eslint.config.js` (it imports
     the deleted file).
  3. `tsconfig.json` must extend `"$app/tsconfig"` instead of
     `./.svelte-kit/tsconfig.json`.
  4. `$lib` is removed (5 `.svelte` files use it). Preferred fix: add
     `"imports": {"#lib": "./src/lib", "#lib/*": "./src/lib/*"}` to
     `package.json` and import `#lib/<file>.ts` (explicit extension for TS;
     `.svelte` keeps its own). Fallback: `alias: { $lib: 'src/lib' }`, which
     works but is deprecated and warns on every run.
  5. Remove the `cookie: ^0.7.2` override: kit 3 depends on `cookie ^2.0.1`
     and the override would force an incompatible major onto it. Without it
     the tree resolved `cookie@2.0.1` and `npm audit` reported 0 findings.
  6. Update the `svelte.config.js` line in `AGENTS.md` (Frontend Architecture).
  Before applying, also smoke-test `vite dev` and `docker build` the frontend
  image, which the trial did not cover.

Still open from earlier rounds:

- **TypeScript is held at `~6.0.3` on purpose**: TypeScript 7 (7.0.2 exists) is
  excluded while `@sveltejs/kit` (even 3.0.0) peers stop at `^6` and
  `typescript-eslint` (8.71.0) requires `<6.1.0`. Re-check both peer ranges
  before moving.

## Revisit on a date

- **Node 26** enters LTS on 2026-10-28 (endoflife.date). After that date it
  becomes a valid target to replace Node 24 in `frontend/Dockerfile`,
  `docker-compose.yml` (frontend-dev) and the CI `node-version`; Node 24 itself
  stays supported until 2028-04, so there is no urgency. Kit 3 needs Node
  `>=22.17`, so either line works with it.

## Not done in the last run

- **`govulncheck` was skipped again** (declined 2026-10-01; it is not
  installed), so the Go modules have never been scanned for known
  vulnerabilities by this skill. Ask again next run.
- Indirect Go modules with newer versions (`testify`, `go-cmp`, `go-internal`,
  `golang.org/x/tools`) were left alone: they only appear in the viper test
  graph, not in the compiled binary.
- `github.com/gorilla/mux` (last release v1.8.1, 2023-10) and
  `github.com/dchest/validator` (single maintainer) are maintenance-risk
  notes, not findings; no replacement was proposed.
