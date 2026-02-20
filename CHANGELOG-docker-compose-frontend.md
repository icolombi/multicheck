# Changelog - Docker Compose Frontend Service

## Date

February 19, 2026

## Summary

Added a frontend service to Docker Compose so the SvelteKit app starts alongside Multicheck and Valkey.
Optimized the frontend workflow with a cached Dockerfile build and optional dev profile.

## Changes

- Added a Node-based frontend service to docker-compose.yml
- Added a frontend Dockerfile to cache dependencies and builds
- Added frontend profiles for production preview and development server
- Documented the frontend URL in README.md
- Updated ARCHITECTURE.md Docker Compose example to include the frontend service

## Notes

- The frontend service uses a production build with `npm run preview` on port 5173.
- The frontend image uses Node 22 LTS.
- The dev profile runs `npm run dev` and reuses a named `node_modules` volume.
