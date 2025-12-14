#!/bin/bash

# Convenience script to run locally the Super-Linter tool to perform
# linting and formatting checks on the codebase as a whole.
#
# Requires a local installation of either Docker or podman.

# Which command to use to run OCI commands; defaults to podman, but
# will switch to Docker if podman is unavailable.
OCI_CLI="podman"
if ! which "${OCI_CLI}" 1>/dev/null 2>&1; then
	OCI_CLI="docker"
fi

# Run Super-Linter locally with the current directory mounted as the
# target
"${OCI_CLI}" run --rm \
	-e GOCACHE=/tmp/.cache/go \
	-e GOMODCACHE=/tmp/.mod-cache \
	-e RUN_LOCAL=true \
	-e DEFAULT_BRANCH="$(git rev-parse --abbrev-ref HEAD)" \
	--user root:root \
	--env-file .super-linter.env \
	-v "$(pwd)":/tmp/lint:Z \
	-v "$(git rev-parse --git-common-dir)":"$(git rev-parse --git-common-dir)":ro \
	ghcr.io/super-linter/super-linter:latest
