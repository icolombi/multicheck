# Changelog - Docker Compose Valkey Healthcheck

## Date

February 19, 2026

## Summary

Added a Valkey healthcheck and enforced startup ordering so Multicheck waits for Valkey to be healthy when running with Docker Compose.

## Changes

- Added a Valkey `healthcheck` using `valkey-cli ping` in docker-compose.yml
- Updated `depends_on` to wait for `service_healthy` before starting Multicheck
- Documented the startup behavior in README.md and ARCHITECTURE.md

## Benefits

- Prevents Multicheck from starting before Valkey is ready
- Reduces transient connection errors during container startup
