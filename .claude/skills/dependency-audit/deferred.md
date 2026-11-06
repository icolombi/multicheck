# Deferred upgrades

Work the dependency-audit skill found but did not apply. The skill reads this
file in Phase 0, **re-verifies every item against current data** (versions,
EOL dates and peer ranges below are a snapshot), and deletes an entry once it
has been applied or is no longer relevant.

Snapshot date: 2026-11-06 (project at v1.6.14). All npm trials were run in
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
  excluded while `@sveltejs/kit` (3.1.1 on 2026-11-06) peers stop at `^6` and
  `typescript-eslint` (8.72.1 on 2026-11-06) requires `<6.1.0`. Re-check both peer ranges
  before moving.

Node moved from 24 to 26 (Active LTS since 2026-10-28) on 2026-10-29 on branch
`chore/deps-2026-10-29`, in `frontend/Dockerfile`, `docker-compose.yml`, the CI
`node-version` and the docs. Verified under `node:26-alpine` with `npm ci`
(lockfile unchanged), `check`, `lint`, `build`, `vite dev` smoke test and an
image build.

## Optional, postponed (last re-checked 2026-11-06)

- **Valkey 9.2** still only `9.2.0-rc2` on 2026-11-06: not a target until GA. The
  floating `9.2` / `9.2-alpine` tags point at the RC: never use them.
  Dev (`docker-compose.yml`) and production (`deploy/azure/variables.tf`) were
  aligned on `9.1.3-alpine` on 2026-10-27; keep them pinned to the same tag.
- **`.nvmrc`** pinning the CI Node line for local work was declined.
- **`minimatch` GHSA-g3hv-75wg-m249** (moderate DoS, `<11.0.1`), reached via
  `eslint`, `@eslint/config-array` and `@typescript-eslint/typescript-estree`,
  all of which still require `^10.2.x` (eslint 10.14.0 on 2026-11-06). Dev-only,
  and the globs come from our own ESLint config. An `overrides.minimatch: ^11.0.3`
  passed check/lint/build in a trial (the APIs ESLint uses, `new Minimatch().match`,
  `minimatch()` and `flipNegate`, survive in 11; `makeRe` was removed) but was
  declined: wait for ESLint to move to minimatch 11, then drop the issue.
- **`node:26-alpine`** is a bare-major tag (`frontend/Dockerfile`,
  `docker-compose.yml`); pinning to `node:26.13-alpine` was declined (again on 2026-11-04).
- **Dependabot `docker-compose` ecosystem** (the `docker` ecosystem does not read
  `docker-compose.yml`, so its Valkey and Node tags are never bumped) was declined.

## Not done in the last run

- **nginx runtime CVEs**: `RUN apk upgrade --no-cache` in the nginx stage of
  `frontend/Dockerfile` covers them, but on 2026-11-06 v1.6.13 shipped the
  `libexpat` of v1.6.12 (4 HIGH) because the step was served from the CI layer
  cache while a local build scanned clean. Fixed in v1.6.14 by the `APK_REFRESH`
  build arg; `ghcr.io/icolombi/multicheck-frontend:1.6.14` on 2026-11-06: 0
  HIGH/CRITICAL (upstream `nginx-unprivileged:1.30-alpine`, Alpine 3.24.2, showed
  1 CRITICAL and 7 HIGH, all fixable). **Scan the image published on GHCR, not a
  local build**, and drop the `apk upgrade` only if it ever breaks a build.
- **No image scan in CI**: nothing scans an image before it is published, which is
  how v1.6.13 went out. Adding trivy to the `build` job was proposed on 2026-11-06,
  not done yet.
- **`govulncheck`** is at `/usr/bin/govulncheck` (on `PATH`). Go 1.27.3 on
  2026-11-06: `No vulnerabilities found.` Keep running it each audit.
- **Local Node** is 26.10.0 on 2026-11-04, the same line as CI, so lockfile
  changes can run on the host. Behind the corporate TLS proxy, containers
  running npm need
  `-e NODE_EXTRA_CA_CERTS=/ca.pem -v /etc/ssl/certs/netskope.pem:/ca.pem:ro`.
- Indirect Go modules with newer versions (`testify`, `go-cmp`, `go-internal`,
  `golang.org/x/tools`, `x/mod`; unchanged on 2026-11-06) were left alone: they only appear in the viper test
  graph, not in the compiled binary. `go.yaml.in/yaml/v3` is the exception: it
  **is** linked (viper's YAML codec), so update it when a release appears
  (v3.0.5 -> v3.0.6 applied on 2026-11-01). `spf13/pflag` is linked too (v1.1.0
  applied on 2026-11-06).
- `github.com/gorilla/mux` (last release v1.8.1, 2023-10) and
  `github.com/dchest/validator` (single maintainer) are maintenance-risk
  notes, not findings; no replacement was proposed.
- `node:26-alpine` (builder only; scanned 2026-11-06, image built 2026-11-05)
  has no HIGH/CRITICAL in its Alpine packages. Six HIGH CVEs sit in npm's own
  bundled packages (`undici` 6.28.0, `brace-expansion` 5.0.9) under
  `/usr/local/lib/node_modules/npm`; nothing from that stage ships, so not
  actionable (same picture as `node:24-alpine` before it).
- `deepmerge` 4.3.1 (GHSA-gcqx-w48m-w2h4, low) comes in through
  `@sveltejs/vite-plugin-svelte`, build-time only; no fixed release exists
  (4.3.1 is the latest). `npm audit --omit=dev` is clean.
