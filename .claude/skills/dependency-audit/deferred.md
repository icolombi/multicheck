# Deferred upgrades

Work the dependency-audit skill found but did not apply. The skill reads this
file in Phase 0, **re-verifies every item against current data** (versions,
EOL dates and peer ranges below are a snapshot), and deletes an entry once it
has been applied or is no longer relevant.

Snapshot date: 2026-10-01 (project at v1.6.5). All npm trials were run in
throwaway copies of `frontend/`: `npm run check`, `lint` and `build` only. Dev
server and runtime behaviour were not exercised.

## Major upgrades (frontend)

None pending. The SvelteKit 3 family (kit 3.0.0, adapter-static 4.0.0,
adapter-auto 8.0.0) was applied on 2026-10-01 on branch `chore/sveltekit-3`:
config moved into `vite.config.ts`, `$lib` -> `#lib` via a `package.json`
`imports` map, `tsconfig.json` extends `$app/tsconfig` and needs its own
`include`/`exclude` (the base no longer has them, so `svelte-check` otherwise
scans `build/`), and the `cookie` override was removed. Verified with `check`,
`lint`, `build`, `vite dev` smoke test and `docker build`.

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

- **`govulncheck`** ran for the first time on 2026-10-01 (v1.7.0, DB updated
  2026-09-28, Go 1.27.1): `No vulnerabilities found.` Keep running it each audit.
- Indirect Go modules with newer versions (`testify`, `go-cmp`, `go-internal`,
  `golang.org/x/tools`) were left alone: they only appear in the viper test
  graph, not in the compiled binary.
- `github.com/gorilla/mux` (last release v1.8.1, 2023-10) and
  `github.com/dchest/validator` (single maintainer) are maintenance-risk
  notes, not findings; no replacement was proposed.
