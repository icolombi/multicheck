# Deferred upgrades

Work the dependency-audit skill found but did not apply. The skill reads this
file in Phase 0, **re-verifies every item against current data** (versions,
EOL dates and peer ranges below are a snapshot), and deletes an entry once it
has been applied or is no longer relevant.

Snapshot date: 2026-10-27 (project at v1.6.9). All npm trials were run in
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
  excluded while `@sveltejs/kit` (3.1.0 on 2026-10-27) peers stop at `^6` and
  `typescript-eslint` (8.72.1) requires `<6.1.0`. Re-check both peer ranges
  before moving.

## Revisit on a date

- **Node 26** enters LTS on 2026-10-28 (endoflife.date). After that date it
  becomes a valid target to replace Node 24 in `frontend/Dockerfile`,
  `docker-compose.yml` (frontend-dev) and the CI `node-version`; Node 24 itself
  stays supported until 2028-04, so there is no urgency. Kit 3 needs Node
  `>=22.17`, so either line works with it. `node:26-alpine` already exists on
  Docker Hub.

## Optional, postponed on 2026-10-27

- **Valkey 9.2** still only `9.2.0-rc1` on 2026-10-27: not a target until GA.
  Dev (`docker-compose.yml`) and production (`deploy/azure/variables.tf`) were
  aligned on `9.1.3-alpine` on 2026-10-27; keep them pinned to the same tag.
- **nginx runtime CVEs**: trivy on `nginxinc/nginx-unprivileged:1.30-alpine`
  (re-scanned 2026-10-27, upstream image still built 2026-10-19) found HIGH CVE-2026-111497 (`brotli-libs` 1.2.0-r1, fixed
  1.2.1-r0) and CVE-2026-92392 (`curl`/`libcurl` 8.22.0-r0, fixed 8.23.0-r0).
  nginx links neither; an `apk upgrade` in `frontend/Dockerfile` was declined in
  favour of waiting for the upstream rebuild (declined again on 2026-10-27).
  Re-scan next run, after a `docker pull` so trivy does not read a stale image.
- **`.nvmrc`** pinning Node 24 for local work was declined.

## Not done in the last run

- **`govulncheck`** is now at `/usr/bin/govulncheck` (on `PATH`). Go 1.27.3 on
  2026-10-27: `No vulnerabilities found.` Keep running it each audit.
- **Local Node** was 26.8.1 (npm 12) on 2026-10-27, not the 24.x used by CI and
  the images, and nvm is **not installed**. Run lockfile-changing `npm install`
  in a `node:24-alpine` container (`-u $(id -u):$(id -g) -e HOME=/tmp -v
  "$PWD:/app" -w /app`), then `npm ci` on the host to restore glibc native
  binaries in `node_modules`. Behind the
  corporate TLS proxy, containers running npm need
  `-e NODE_EXTRA_CA_CERTS=/ca.pem -v /etc/ssl/certs/netskope.pem:/ca.pem:ro`.
- Indirect Go modules with newer versions (`testify`, `go-cmp`, `go-internal`,
  `golang.org/x/tools`, `x/mod`) were left alone: they only appear in the viper test
  graph, not in the compiled binary.
- `github.com/gorilla/mux` (last release v1.8.1, 2023-10) and
  `github.com/dchest/validator` (single maintainer) are maintenance-risk
  notes, not findings; no replacement was proposed.
- `node:24-alpine` (builder only) carries HIGH CVEs in npm's own bundled
  packages (`undici`, `brace-expansion`, `http-cache-semantics`); nothing from
  that stage ships, so not actionable.
