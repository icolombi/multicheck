# Deferred upgrades

Work the dependency-audit skill found but did not apply. The skill reads this
file in Phase 0, **re-verifies every item against current data** (versions,
EOL dates and peer ranges below are a snapshot), and deletes an entry once it
has been applied or is no longer relevant.

Snapshot date: 2026-10-31 (project at v1.6.10). All npm trials were run in
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
  excluded while `@sveltejs/kit` (3.1.1 on 2026-10-31) peers stop at `^6` and
  `typescript-eslint` (8.72.1 on 2026-10-31) requires `<6.1.0`. Re-check both peer ranges
  before moving.

Node moved from 24 to 26 (Active LTS since 2026-10-28) on 2026-10-29 on branch
`chore/deps-2026-10-29`, in `frontend/Dockerfile`, `docker-compose.yml`, the CI
`node-version` and the docs. Verified under `node:26-alpine` with `npm ci`
(lockfile unchanged), `check`, `lint`, `build`, `vite dev` smoke test and an
image build.

## Optional, postponed on 2026-10-31

- **Valkey 9.2** still only `9.2.0-rc1` on 2026-10-31: not a target until GA.
  Dev (`docker-compose.yml`) and production (`deploy/azure/variables.tf`) were
  aligned on `9.1.3-alpine` on 2026-10-27; keep them pinned to the same tag.
- **nginx runtime CVEs**: trivy on `nginxinc/nginx-unprivileged:1.30-alpine`
  (re-scanned 2026-10-31 after `docker pull`, upstream image still built 2026-10-19) found
  CVE-2026-111497 (`brotli-libs` 1.2.0-r1, fixed 1.2.1-r0), **re-rated
  CRITICAL** (was HIGH), and HIGH CVE-2026-92392 (`curl`/`libcurl` 8.22.0-r0,
  fixed 8.23.0-r0). nginx links neither; an `apk upgrade` in
  `frontend/Dockerfile` was declined in favour of waiting for the upstream
  rebuild (declined again on 2026-10-27, 2026-10-29 and 2026-10-31).
  Re-scan next run, after a `docker pull` so trivy does not read a stale image.
- **`.nvmrc`** pinning the CI Node line for local work was declined.
- **`minimatch` GHSA-g3hv-75wg-m249** (moderate DoS, `<11.0.1`), reached via
  `eslint`, `@eslint/config-array` and `@typescript-eslint/typescript-estree`,
  all of which still require `^10.2.x` (eslint 10.14.0 on 2026-10-31). Dev-only,
  and the globs come from our own ESLint config. An `overrides.minimatch: ^11.0.3`
  passed check/lint/build in a trial (the APIs ESLint uses, `new Minimatch().match`,
  `minimatch()` and `flipNegate`, survive in 11; `makeRe` was removed) but was
  declined: wait for ESLint to move to minimatch 11, then drop the issue.
- **`node:26-alpine`** is a bare-major tag (`frontend/Dockerfile`,
  `docker-compose.yml`); pinning to `node:26.13-alpine` was declined.
- **Dependabot `docker-compose` ecosystem** (the `docker` ecosystem does not read
  `docker-compose.yml`, so its Valkey and Node tags are never bumped) was declined.

## Not done in the last run

- **`govulncheck`** is at `/usr/bin/govulncheck` (on `PATH`). Go 1.27.3 on
  2026-10-31: `No vulnerabilities found.` Keep running it each audit.
- **Local Node** is 26.10.0 on 2026-10-31, the same line as CI, so lockfile
  changes can run on the host. Behind the corporate TLS proxy, containers
  running npm need
  `-e NODE_EXTRA_CA_CERTS=/ca.pem -v /etc/ssl/certs/netskope.pem:/ca.pem:ro`.
- Indirect Go modules with newer versions (`testify`, `go-cmp`, `go-internal`,
  `golang.org/x/tools`, `x/mod`; unchanged on 2026-10-31) were left alone: they only appear in the viper test
  graph, not in the compiled binary.
- `github.com/gorilla/mux` (last release v1.8.1, 2023-10) and
  `github.com/dchest/validator` (single maintainer) are maintenance-risk
  notes, not findings; no replacement was proposed.
- `node:26-alpine` (builder only; scanned 2026-10-31, image built 2026-10-28)
  has no HIGH/CRITICAL in its Alpine packages. Six HIGH CVEs sit in npm's own
  bundled packages (`undici` 6.28.0, `brace-expansion` 5.0.9) under
  `/usr/local/lib/node_modules/npm`; nothing from that stage ships, so not
  actionable (same picture as `node:24-alpine` before it).
- `deepmerge` 4.3.1 (GHSA-gcqx-w48m-w2h4, low) comes in through
  `@sveltejs/vite-plugin-svelte`, build-time only; no fixed release exists
  (4.3.1 is the latest). `npm audit --omit=dev` is clean.
