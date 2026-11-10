# Stack inventory

<!-- Maintained by the stack-audit skill. Safe to edit by hand: keep the headings and the table columns. -->

- Last full audit: 2026-11-06
- Last audit: 2026-11-10 (incremental; upstream data reused, vulnerability scans rerun)
- Commit audited: `2d1a085`

## Components

| Component | Category | In use | Declared in | Latest | Recommended | EOL | Status | Source | Checked |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Go | runtime | 1.27.3 | `go.mod:3`, `Dockerfile:1` | 1.27.3 | 1.27 (no LTS, newest stable) | none announced | OK | endoflife.date | 2026-11-06 |
| Node.js (`node:26-alpine`) | runtime / builder image | 26 (tag resolves to 26.13.0) | `frontend/Dockerfile:1`, `docker-compose.yml`, `.github/workflows/docker-publish.yml` | 26.13.0 | 26 (LTS) | 2029-04-30 | VULN (trivy 2026-11-10: 6 HIGH in npm's bundled `undici` 6.28.0, `brace-expansion` 5.0.9, all fixable upstream; image rebuilt 2026-11-05 without them; builder stage only, nothing ships) | endoflife.date, trivy | 2026-11-06 |
| `golang:1.27` (Debian 13.7) | builder image | 1.27 | `Dockerfile:1` | 1.27.3 | 1.27 | Debian 13: 2030-06-30 | VULN (trivy 2026-11-10: 159 HIGH/CRITICAL matches, 65 unique CVEs, 1 CRITICAL in `openssh-client` with no Debian fix yet, 2 fixable in `zlib1g` via `+deb13u1`; builder stage only, nothing ships) | endoflife.date, trivy | 2026-11-06 |
| Alpine (backend runtime) | base image | 3.24 | `Dockerfile:23` | 3.24.4 | 3.24 | 2028-06-01 | OK | endoflife.date, Docker Hub, trivy | 2026-11-06 |
| `nginxinc/nginx-unprivileged` | base image | 1.30-alpine (Alpine 3.24.2, built 2026-10-19) | `frontend/Dockerfile:11` | 1.30.6 stable, 1.31.8 mainline | 1.30 (stable, project policy) | none announced | VULN (trivy 2026-11-10: 1 CRITICAL in `brotli-libs`, 7 HIGH unique (`libexpat`, `musl`, `zlib`, `curl`), upstream image still from 2026-10-19, all fixed in Alpine 3.24; covered at build time by `apk upgrade`) | endoflife.date, Docker Hub, trivy | 2026-11-06 |
| `multicheck-frontend` published image | release image | 1.6.14 (live `/health` checked 2026-11-10) | `deploy/azure/terraform.tfvars` | 1.6.14 | 1.6.14 | n/a | OK (trivy 2026-11-10: 0 HIGH/CRITICAL; 1.6.13 is skipped: same CVEs as 1.6.12) | trivy on GHCR image | 2026-11-06 |
| `multicheck-backend` published image | release image | 1.6.14 (live `/health` checked 2026-11-10) | `deploy/azure/terraform.tfvars` | 1.6.14 | 1.6.14 | n/a | OK (trivy 2026-11-10: 0 HIGH/CRITICAL) | trivy on GHCR image | 2026-11-06 |
| Valkey | service | 9.1.3-alpine | `docker-compose.yml`, `deploy/azure/variables.tf:45` | 9.1.3 (9.2.0-rc2 is a pre-release) | 9.1 | 2031-05-19 | OK | endoflife.date, GitHub releases, trivy | 2026-11-06 |
| SvelteKit | framework | 3.1.1 | `frontend/package.json` | 3.1.1 | 3.1 | not tracked | OK | npm | 2026-11-06 |
| Svelte | framework | 5.57.3 | `frontend/package.json` | 5.57.3 | 5 | none announced | OK | endoflife.date, npm | 2026-11-06 |
| Vite | build tool | 8.3.6 | `frontend/package.json` | 8.3.6 | 8.3.6 | not tracked | OK | npm | 2026-11-06 |
| Tailwind CSS | framework | 4.3.5 | `frontend/package.json` | 4.3.5 | 4.3.5 | none announced | OK | endoflife.date, npm | 2026-11-06 |
| TypeScript | toolchain | 6.0.3 | `frontend/package.json` | 7.0.2 | 6.0 (kit peers `^6.0.0`, typescript-eslint `<6.1.0`) | not tracked | OK (accepted) | npm | 2026-11-06 |
| ESLint | toolchain | 10.14.0 | `frontend/package.json` | 10.14.0 | 10 | none announced | OK | endoflife.date, npm | 2026-11-06 |
| `gorilla/mux` | library | 1.8.1 | `go.mod` | 1.8.1 | 1.8.1 | not tracked | OK (last commit 2024-06, not archived) | GitHub | 2026-11-06 |
| `gomodule/redigo` | library | 1.9.3 | `go.mod` | 1.9.3 | 1.9.3 | not tracked | OK | GitHub | 2026-11-06 |
| `spf13/viper` | library | 1.21.0 | `go.mod` | 1.21.0 | 1.21.0 | not tracked | OK | GitHub | 2026-11-06 |
| `dchest/validator` | library | 1.0.0 | `go.mod` | 1.0.0 | 1.0.0 | not tracked | OK | Go proxy | 2026-11-06 |
| `spf13/pflag` | library (indirect, linked) | 1.1.0 | `go.mod` | 1.1.0 | 1.1.0 | not tracked | OK | Go proxy | 2026-11-06 |
| `actions/checkout` | CI | v7 | `.github/workflows/docker-publish.yml` | v7.0.1 | v7 | n/a | OK | GitHub | 2026-11-06 |
| `actions/setup-go` | CI | v7 | `.github/workflows/docker-publish.yml` | v7.0.0 | v7 | n/a | OK | GitHub | 2026-11-06 |
| `actions/setup-node` | CI | v7 | `.github/workflows/docker-publish.yml` | v7.1.0 | v7 | n/a | OK | GitHub | 2026-11-06 |
| `docker/setup-buildx-action` | CI | v4 | `.github/workflows/docker-publish.yml` | v4.4.1 | v4 | n/a | OK | GitHub | 2026-11-06 |
| `docker/login-action` | CI | v4 | `.github/workflows/docker-publish.yml` | v4.8.0 | v4 | n/a | OK | GitHub | 2026-11-06 |
| `docker/metadata-action` | CI | v6 | `.github/workflows/docker-publish.yml` | v6.2.0 | v6 | n/a | OK | GitHub | 2026-11-06 |
| `docker/build-push-action` | CI | v7 | `.github/workflows/docker-publish.yml` | v7.4.0 | v7 | n/a | OK | GitHub | 2026-11-06 |
| GitHub runner | CI | `ubuntu-24.04` | `.github/workflows/docker-publish.yml` | 26.04 | 24.04 (actionlint does not know `ubuntu-26.04` yet) | 2029-05-31 | OK | endoflife.date, actions/runner-images | 2026-11-06 |
| Terraform CLI | IaC | >= 1.17.0 (local 1.17.1) | `deploy/azure/versions.tf:2` | 1.17.1 | 1.17 | none announced | OK | endoflife.date | 2026-11-06 |
| `hashicorp/azurerm` | IaC provider | 5.13.0 | `deploy/azure/versions.tf`, `.terraform.lock.hcl` | 5.13.0 | 5.13 | not tracked | OK | Terraform Registry | 2026-11-06 |
| `azure/azapi` | IaC provider | 2.13.0 | `deploy/azure/versions.tf`, `.terraform.lock.hcl` | 2.13.0 | 2.13 | not tracked | OK | Terraform Registry | 2026-11-06 |
| `Microsoft.App/containerApps` API | managed service | 2026-07-01 | `deploy/azure/main.tf:188` | 2026-07-01 | 2026-07-01 | not announced | OK (bumped 2026-11-08; plan: 1 in-place change, body unchanged) | azure-rest-api-specs | 2026-11-06 |

## Dependency groups

| Group | Manifest | Direct deps | Behind (major) | Vulnerable | Checked |
| --- | --- | --- | --- | --- | --- |
| npm | `frontend/package.json` | 24 | 1 (TypeScript, accepted) | 0 HIGH/CRITICAL; 10 moderate, 5 low, all dev-only (`npm audit --omit=dev` clean) | 2026-11-10 |
| Go modules | `go.mod` | 4 | 0 | 0 (`govulncheck`, trivy) | 2026-11-10 |

## Accepted exceptions

Deliberate choices the audit should keep showing but not re-propose. Remove a row when it no longer applies.

Decisions were carried over from `.claude/skills/dependency-audit/deferred.md`; the review dates were set by the 2026-11-06 audit.

| Component | Stay on | Reason | Decided | Review by |
| --- | --- | --- | --- | --- |
| TypeScript | 6.0 | `@sveltejs/kit` peers `^6.0.0` and `typescript-eslint` requires `<6.1.0` | 2026-11-04 | 2027-02-06 |
| `node:26-alpine` tag | bare major | Pinning to a minor was declined | 2026-11-04 | 2027-05-06 |
| `.nvmrc` | not added | Declined | 2026-11-04 | 2027-05-06 |
| Dependabot `docker-compose` ecosystem | not configured | Declined; compose tags are bumped during audits | 2026-11-04 | 2027-05-06 |
| `minimatch` GHSA-g3hv-75wg-m249 | 10.2.x, no override | Moderate, dev-only, globs come from the project's own ESLint config; wait for ESLint to move to minimatch 11 | 2026-11-04 | 2027-02-06 |

## Manifests

Fingerprints used by `scan-manifests.sh --check` to detect changes.

| Path | Hash |
| --- | --- |
| `.github/dependabot.yml` | `adfd07108e3f` |
| `.github/workflows/docker-publish.yml` | `024ddf5837e6` |
| `Dockerfile` | `cd75fe4aecaa` |
| `deploy/azure/.terraform.lock.hcl` | `0df24571c01d` |
| `deploy/azure/main.tf` | `d780ac1c2f0a` |
| `deploy/azure/outputs.tf` | `24fc4a8b07d8` |
| `deploy/azure/variables.tf` | `186696245080` |
| `deploy/azure/versions.tf` | `dfd8ef851f03` |
| `docker-compose.yml` | `45d2a87d4daa` |
| `frontend/Dockerfile` | `67789de5e82d` |
| `frontend/package-lock.json` | `2bfd46eba862` |
| `frontend/package.json` | `d95b076b78f0` |
| `go.mod` | `c23c9f90a9da` |
