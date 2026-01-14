#!/bin/bash
export PODMAN_IGNORE_CGROUPSV1_WARNING=1
# Stop and remove existing containers first (suppress all output if containers don't exist)
podman-compose down --remove-orphans >/dev/null 2>&1 || true
# Then build and start with force recreate
podman-compose up --build --force-recreate