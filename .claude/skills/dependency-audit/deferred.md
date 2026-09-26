# Deferred upgrades

Work the dependency-audit skill found but did not apply. The skill reads this
file in Phase 0, **re-verifies every item against current data** (versions,
EOL dates and peer ranges below are a snapshot), and deletes an entry once it
has been applied or is no longer relevant.

Snapshot date: 2026-09-26 (project at v1.6.4). All npm trials were run in
throwaway copies of `frontend/`: `npm run check`, `lint` and `build` only. Dev
server and runtime behaviour were not exercised.

## Major upgrades (frontend)

None pending. The 2026-09-26 majors (ESLint 10 stack, Vite 8 + plugin-svelte 7,
prettier-plugin-svelte 4, svelte-sonner 1, TypeScript 6.0) were applied in the
v1.6.4 round.

Still open from that round:

- **`svelte-sonner` 1.x toasts were not checked visually** (type-check, build
  and a `vite dev` smoke test only). Look at a success and an error toast, in
  light and dark mode.
- **TypeScript is held at `~6.0.3` on purpose**: TypeScript 7 is excluded while
  `@sveltejs/kit` peers stop at 6 and `typescript-eslint` requires `<6.1.0`.
  Re-check both peer ranges before moving (`~6.0` -> `^6` or 7).

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
