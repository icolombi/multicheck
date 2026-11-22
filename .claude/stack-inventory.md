# Stack inventory

<!-- Maintained by the stack-audit skill. Safe to edit by hand: keep the headings and the table columns. -->

- Last full audit: 2026-11-06
- Last audit: 2026-11-22 (incremental; no manifest changed, all upstream data refreshed; batches 1-2 applied (npm minors, `adapter-auto` removed, Valkey 9.1.4, azapi 2.14); govulncheck, npm audit, trivy fs and image scans rerun)
- Commit audited: `9da3e44`

## Components

| Component | Category | In use | Declared in | Latest | Recommended | EOL | Status | Source | Checked |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Go | runtime | 1.27.4 | `go.mod:3`, `Dockerfile:1` (`golang:1.27`), CI via `go-version-file` | 1.27.4 | 1.27.4 (no LTS, newest stable) | none announced | OK (govulncheck clean) | endoflife.date, go.dev, govulncheck | 2026-11-22 |
| Node.js (`node:26-alpine`) | runtime / builder image | 26 (tag resolves to 26.13.1) | `frontend/Dockerfile:1`, `docker-compose.yml:40`, `.github/workflows/docker-publish.yml:63` | 26.13.1 | 26 (LTS) | 2029-04-30 | VULN (trivy 2026-11-22: 6 HIGH in npm's bundled `undici` and `brace-expansion`, all fixable upstream; builder stage only, nothing ships) | endoflife.date, trivy | 2026-11-22 |
| `golang:1.27` (Debian 13.7) | builder image | 1.27 | `Dockerfile:1` | 1.27.4 | 1.27 | Debian 13: 2030-06-30 | VULN (trivy 2026-11-22: 232 HIGH/CRITICAL matches, 121 unique CVEs, 2 CRITICAL (`openssh-client` no Debian fix, `linux-libc-dev`), 32 fixable; builder stage only, nothing ships) | endoflife.date, trivy | 2026-11-22 |
| Alpine (backend runtime) | base image | 3.24 | `Dockerfile:23` | 3.24.4 | 3.24 | 2028-06-01 | OK (trivy 2026-11-22: 0 HIGH/CRITICAL) | endoflife.date, trivy | 2026-11-22 |
| `nginxinc/nginx-unprivileged` | base image | 1.30-alpine (nginx 1.30.6, Alpine 3.24.2, tag last rebuilt 2026-10-19) | `frontend/Dockerfile:11` | nginx 1.30.7 stable, 1.31.9 mainline (2026-11-18); no image tag with either yet | 1.30 (stable, project policy) | none announced | VULN (trivy 2026-11-22: 5 CRITICAL, 18 unique HIGH/CRITICAL; 6 in `nginx` itself fixed only in 1.30.7, not reachable by `apk upgrade`; rest Alpine packages fixed in 3.24) | endoflife.date, nginx.org CHANGES, Docker Hub, trivy | 2026-11-22 |
| `multicheck-frontend` published image | release image | 1.6.15 (deployed 2026-11-14; app stopped since 2026-11-10) | `deploy/azure/terraform.tfvars:10` | 1.6.15 | next release on nginx 1.30.7 | n/a | VULN (trivy 2026-11-22: 4 CRITICAL + 2 HIGH in `nginx` 1.30.6-r1 (HTTP/2, HTTP/3, mail, resolver, TLS: none enabled in `default.conf.template`), 1 HIGH `libxml2` 2.13.9-r2 fixed in -r3) | trivy on GHCR image | 2026-11-22 |
| `multicheck-backend` published image | release image | 1.6.15 (stdlib go1.27.4; deployed 2026-11-14; app stopped since 2026-11-10) | `deploy/azure/terraform.tfvars:10` | 1.6.15 | 1.6.15 | n/a | OK (trivy 2026-11-22: 0 HIGH/CRITICAL) | trivy on GHCR image | 2026-11-22 |
| Valkey | service | 9.1.4-alpine | `docker-compose.yml:13`, `deploy/azure/variables.tf:45` | 9.2.0 (9.1.4 patch, both 2026-11-18) | 9.1.4 (9.2 is a minor, optional) | 9.1: 2031-05-19 | OK (bumped 2026-11-22; trivy: 0 HIGH/CRITICAL) | endoflife.date, GitHub releases, trivy | 2026-11-22 |
| SvelteKit | framework | 3.4.0 | `frontend/package.json` | 3.4.0 | 3.4 (still peers TypeScript `^6.0.0`) | not tracked | OK | npm | 2026-11-22 |
| Svelte | framework | 5.59.1 | `frontend/package.json` | 5.59.1 | 5 | none announced | OK | endoflife.date, npm | 2026-11-22 |
| Vite | build tool | 8.3.8 | `frontend/package.json` | 8.3.8 | 8.3.8 | not tracked | OK | npm | 2026-11-22 |
| Tailwind CSS | framework | 4.3.5 | `frontend/package.json` | 4.3.5 | 4.3.5 | none announced | OK | endoflife.date, npm | 2026-11-22 |
| TypeScript | toolchain | 6.0.3 | `frontend/package.json` | 7.0.2 | 6.0 (kit 3.4.0 still peers `^6.0.0`, typescript-eslint 8.73.0 `<6.1.0`) | not tracked | OK (accepted) | npm | 2026-11-22 |
| ESLint | toolchain | 10.15.0 | `frontend/package.json` | 10.15.0 | 10 | none announced | OK | endoflife.date, npm | 2026-11-22 |
| `gorilla/mux` | library | 1.8.1 | `go.mod` | 1.8.1 | 1.8.1 | not tracked | OK (last push 2024-08, not archived) | GitHub | 2026-11-22 |
| `gomodule/redigo` | library | 1.9.3 | `go.mod` | 1.9.3 | 1.9.3 | not tracked | OK | GitHub | 2026-11-22 |
| `spf13/viper` | library | 1.21.0 | `go.mod` | 1.21.0 | 1.21.0 | not tracked | OK | Go proxy | 2026-11-22 |
| `dchest/validator` | library | 1.0.0 | `go.mod` | 1.0.0 | 1.0.0 | not tracked | OK | Go proxy | 2026-11-22 |
| `spf13/pflag` | library (indirect, linked) | 1.1.0 | `go.mod` | 1.1.0 | 1.1.0 | not tracked | OK | Go proxy | 2026-11-22 |
| `actions/checkout` | CI | v7 | `.github/workflows/docker-publish.yml` | v7.0.1 | v7 | n/a | OK | GitHub | 2026-11-22 |
| `actions/setup-go` | CI | v7 | `.github/workflows/docker-publish.yml` | v7.1.0 | v7 | n/a | OK | GitHub | 2026-11-22 |
| `actions/setup-node` | CI | v7 | `.github/workflows/docker-publish.yml` | v7.1.0 | v7 | n/a | OK | GitHub | 2026-11-22 |
| `docker/setup-buildx-action` | CI | v4 | `.github/workflows/docker-publish.yml` | v4.4.1 | v4 | n/a | OK | GitHub | 2026-11-22 |
| `docker/login-action` | CI | v4 | `.github/workflows/docker-publish.yml` | v4.8.0 | v4 | n/a | OK | GitHub | 2026-11-22 |
| `docker/metadata-action` | CI | v6 | `.github/workflows/docker-publish.yml` | v6.2.0 | v6 | n/a | OK | GitHub | 2026-11-22 |
| `docker/build-push-action` | CI | v7 | `.github/workflows/docker-publish.yml` | v7.4.0 | v7 | n/a | OK | GitHub | 2026-11-22 |
| GitHub runner | CI | `ubuntu-24.04` | `.github/workflows/docker-publish.yml` | 26.04 | 24.04 (actionlint 1.7.12, latest, still rejects `ubuntu-26.04`) | 2029-05-31 | OK | endoflife.date, actionlint | 2026-11-22 |
| Terraform CLI | IaC | >= 1.17.0 (local 1.17.2) | `deploy/azure/versions.tf:2` | 1.17.2 | 1.17 | none announced | OK | endoflife.date | 2026-11-22 |
| `hashicorp/azurerm` | IaC provider | 5.14.0 | `deploy/azure/versions.tf` (`~> 5.14`), `.terraform.lock.hcl` | 5.14.0 | 5.14 | not tracked | OK (plan 2026-11-14: no changes) | Terraform Registry | 2026-11-22 |
| `azure/azapi` | IaC provider | 2.14.0 | `deploy/azure/versions.tf` (`~> 2.14`), `.terraform.lock.hcl` | 2.14.0 | 2.14 | not tracked | OK (`terraform validate` clean) | Terraform Registry | 2026-11-22 |
| `Microsoft.App/containerApps` API | managed service | 2026-07-01 | `deploy/azure/main.tf:188` | 2026-07-01 | 2026-07-01 | not announced | OK (not rechecked 2026-11-22) | azure-rest-api-specs | 2026-11-14 |

## Dependency groups

| Group | Manifest | Direct deps | Behind (major) | Vulnerable | Checked |
| --- | --- | --- | --- | --- | --- |
| npm | `frontend/package.json` | 23 (`@sveltejs/adapter-auto` removed 2026-11-22, unused) | 1 (TypeScript, accepted) | 0 (`npm audit` clean, dev included) | 2026-11-22 |
| Go modules | `go.mod` | 4 | 0 | 0 (`govulncheck` on go1.27.4, trivy) | 2026-11-22 |

## Accepted exceptions

Deliberate choices the audit should keep showing but not re-propose. Remove a row when it no longer applies.

Decisions were carried over from `.claude/skills/dependency-audit/deferred.md`; the review dates were set by the 2026-11-06 audit.

| Component | Stay on | Reason | Decided | Review by |
| --- | --- | --- | --- | --- |
| TypeScript | 6.0 | `@sveltejs/kit` peers `^6.0.0` and `typescript-eslint` requires `<6.1.0` | 2026-11-04 | 2027-02-06 |
| `node:26-alpine` tag | bare major | Pinning to a minor was declined | 2026-11-04 | 2027-05-06 |
| `.nvmrc` | not added | Declined | 2026-11-04 | 2027-05-06 |
| Dependabot `docker-compose` ecosystem | not configured | Declined; compose tags are bumped during audits | 2026-11-04 | 2027-05-06 |

## Manifests

Fingerprints used by `scan-manifests.sh --check` to detect changes.

| Path | Hash |
| --- | --- |
| `.github/dependabot.yml` | `adfd07108e3f` |
| `.github/workflows/docker-publish.yml` | `024ddf5837e6` |
| `Dockerfile` | `cd75fe4aecaa` |
| `deploy/azure/.terraform.lock.hcl` | `d251961c22c5` |
| `deploy/azure/main.tf` | `d780ac1c2f0a` |
| `deploy/azure/outputs.tf` | `24fc4a8b07d8` |
| `deploy/azure/variables.tf` | `7372eb4d369d` |
| `deploy/azure/versions.tf` | `728a5d207309` |
| `docker-compose.yml` | `8e060dfedfee` |
| `frontend/Dockerfile` | `67789de5e82d` |
| `frontend/package-lock.json` | `3bdcd017ae17` |
| `frontend/package.json` | `6d722e7bd4a8` |
| `go.mod` | `9686d9a4965b` |
