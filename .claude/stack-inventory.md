# Stack inventory

<!-- Maintained by the stack-audit skill. Safe to edit by hand: keep the headings and the table columns. -->

- Last full audit: 2026-11-06
- Last audit: 2026-11-28 (incremental; no manifest changed; lifecycle data reused from 2026-11-22/25; base image tags, Valkey, nginx and npm refreshed; govulncheck, npm audit, trivy fs and image scans rerun; batch 1 (npm patches/minors) and batch 2 (release 1.6.17, rebuilt images scanned on GHCR) applied)
- Commit audited: `ffe76fa`

## Components

| Component | Category | In use | Declared in | Latest | Recommended | EOL | Status | Source | Checked |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Go | runtime | 1.27.4 | `go.mod:3`, `Dockerfile:1` (`golang:1.27`), CI via `go-version-file` | 1.27.4 | 1.27.4 (no LTS, newest stable) | none announced | OK (govulncheck clean) | endoflife.date, go.dev, govulncheck | 2026-11-22 |
| Node.js (`node:26-alpine`) | runtime / builder image | 26 (tag resolves to 26.13.1, Alpine 3.24.4, tag rebuilt 2026-11-11) | `frontend/Dockerfile:1`, `docker-compose.yml:40`, `.github/workflows/docker-publish.yml:63` | 26.13.1 | 26 (LTS) | eoas 2027-10-27, EOL 2029-04-30 | VULN (trivy 2026-11-28: unchanged, 6 HIGH in npm's bundled `undici` and `brace-expansion`, fixable upstream only; builder stage only, nothing ships) | endoflife.date, Docker Hub, trivy | 2026-11-28 |
| `golang:1.27` (Debian 13.8) | builder image | 1.27 | `Dockerfile:1` | 1.27.4 | 1.27 | Debian 13: 2030-06-30 | VULN (trivy 2026-11-28, tag rebuilt 2026-11-24: 128 HIGH/CRITICAL matches, 57 unique CVEs, 1 CRITICAL (`openssh-client`, no fix), 1 fixable; builder stage only, nothing ships) | endoflife.date, Docker Hub, trivy | 2026-11-28 |
| Alpine (backend runtime) | base image | 3.24 | `Dockerfile:23` | 3.24.4 | 3.24 | 2028-06-01 | OK (trivy 2026-11-22: 0 HIGH/CRITICAL) | endoflife.date, trivy | 2026-11-22 |
| `nginxinc/nginx-unprivileged` | base image | 1.30-alpine (nginx 1.30.6, Alpine 3.24.2, tag last rebuilt 2026-10-19) | `frontend/Dockerfile:11` | nginx 1.30.8 stable (2026-11-23, fixes `ssl_early_data` CPU DoS); newest image tags still 1.30.6 / 1.31.7 (2026-10-19), checked 2026-11-28 | 1.30 (stable, project policy) | none announced | VULN (trivy 2026-11-28: 6 in `nginx` 1.30.6 fixed only in 1.30.7+, not reachable by `apk upgrade` (the image uses nginx.org's package, not Alpine's 1.30.8-r0); the 20 in Alpine packages are fixed by the build's `apk upgrade`) | endoflife.date, nginx.org CHANGES, Docker Hub, trivy, pkgs.alpinelinux.org | 2026-11-28 |
| `multicheck-frontend` published image | release image | 1.6.17 (deployed 2026-11-28, revision `--0000010`, `/api/health` reports v1.6.17) | `deploy/azure/terraform.tfvars:10` | 1.6.17 | next release on nginx 1.30.8 | n/a | VULN (trivy 2026-11-28 on 1.6.17: 4 CRITICAL + 2 HIGH, all in `nginx` 1.30.6-r1 (HTTP/2, HTTP/3, mail, resolver, TLS: none used by `frontend/nginx/*.conf*`); the 6 `libxml2` HIGH of 1.6.16 are fixed by the rebuild) | trivy on GHCR image, pkgs.alpinelinux.org | 2026-11-28 |
| `multicheck-backend` published image | release image | 1.6.17 (stdlib go1.27.4, Alpine 3.24.4) | `deploy/azure/terraform.tfvars:10` | 1.6.17 | 1.6.17 | n/a | OK (trivy 2026-11-28: 0 HIGH/CRITICAL) | trivy on GHCR image | 2026-11-28 |
| Valkey | service | 9.1.4-alpine | `docker-compose.yml:13`, `deploy/azure/variables.tf:45` | 9.2.1 (2026-11-25, urgency MODERATE, bug fixes only); 9.1.4 still newest 9.1 | 9.1.4 (9.2 is a minor, optional) | 9.1: 2031-05-19 | OK (trivy 2026-11-28: 0 HIGH/CRITICAL) | endoflife.date, GitHub releases, trivy | 2026-11-28 |
| SvelteKit | framework | 3.4.1 | `frontend/package.json` | 3.4.1 | 3.4 (still peers TypeScript `^6.0.0`) | not tracked | OK | npm | 2026-11-28 |
| Svelte | framework | 5.59.2 | `frontend/package.json` | 5.59.2 | 5.59.2 | none announced | OK | endoflife.date, npm | 2026-11-25 |
| Vite | build tool | 8.4.0 | `frontend/package.json` | 8.4.0 | 8.4.0 | not tracked | OK | npm | 2026-11-28 |
| Tailwind CSS | framework | 4.3.5 | `frontend/package.json` | 4.3.5 | 4.3.5 | none announced | OK | endoflife.date, npm | 2026-11-22 |
| TypeScript | toolchain | 6.0.3 | `frontend/package.json` | 7.0.2 | 6.0 (kit 3.4.0 still peers `^6.0.0`, typescript-eslint 8.73.0 `<6.1.0`) | not tracked | OK (accepted) | npm | 2026-11-22 |
| ESLint | toolchain | 10.16.0 | `frontend/package.json` | 10.16.0 | 10 | none announced | OK | endoflife.date, npm | 2026-11-28 |
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
| `Microsoft.App/containerApps` API | managed service | 2026-07-01 | `deploy/azure/main.tf:188` | 2026-07-01 | 2026-07-01 | not announced | OK | azure-rest-api-specs | 2026-11-25 |

## Dependency groups

| Group | Manifest | Direct deps | Behind (major) | Vulnerable | Checked |
| --- | --- | --- | --- | --- | --- |
| npm | `frontend/package.json` | 23 | 1 (TypeScript, accepted); `@lucide/svelte` 1.83.0, `@sveltejs/kit` 3.4.1, `eslint` 10.16.0, `eslint-config-prettier` 10.1.9, `postcss` 8.5.34, `vite` 8.4.0 applied 2026-11-28 | 0 (`npm audit` clean, dev included) | 2026-11-28 |
| Go modules | `go.mod` | 4 | 0 | 0 (`govulncheck` on go1.27.4, trivy) | 2026-11-28 |

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
| `frontend/Dockerfile` | `a3094d2fbdd7` |
| `frontend/package-lock.json` | `e7690015cf29` |
| `frontend/package.json` | `467612556a68` |
| `go.mod` | `9686d9a4965b` |
