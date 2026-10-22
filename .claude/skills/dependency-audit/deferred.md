# Deferred upgrades

Work the dependency-audit skill found but did not apply. The skill reads this
file in Phase 0, **re-verifies every item against current data** (versions,
EOL dates and peer ranges below are a snapshot), and deletes an entry once it
has been applied or is no longer relevant.

Snapshot date: 2026-10-22 (project at v1.6.8). All npm trials were run in
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

## Optional, postponed on 2026-10-22

- **Valkey image tag** `valkey/valkey:9` in `docker-compose.yml` is a floating
  major; pinning to `9.1` (9.1.2, EOL 2031-05) was declined again. Note that the
  `9.2` tag existed but pointed at `9.2.0-rc1` on 2026-10-22: not a target until
  a 9.2.0 GA is tagged.
- **TypeScript 7.0.2** still blocked: `@sveltejs/kit` peers `^6.0.0`,
  `typescript-eslint` 8.72.0 requires `<6.1.0`.

## Not done in the last run

- **`govulncheck`** is installed at `~/go/bin/govulncheck` (not on the bash
  `PATH`, call it by full path). Go 1.27.3 on 2026-10-22: `No vulnerabilities
  found.` Keep running it each audit.
- **Local Node** is now 24.22.1 via `nvm.fish` (npm 11), matching CI and the
  images, so `npm install` no longer churns `package-lock.json`. Behind the
  corporate TLS proxy, containers running npm need
  `-e NODE_EXTRA_CA_CERTS=/ca.pem -v /etc/ssl/certs/netskope.pem:/ca.pem:ro`.
- Indirect Go modules with newer versions (`testify`, `go-cmp`, `go-internal`,
  `golang.org/x/tools`, `x/mod`, `x/sync`) were left alone: they only appear in the viper test
  graph, not in the compiled binary.
- `github.com/gorilla/mux` (last release v1.8.1, 2023-10) and
  `github.com/dchest/validator` (single maintainer) are maintenance-risk
  notes, not findings; no replacement was proposed.
