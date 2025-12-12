#!/bin/bash

# Convenience script to build a development-focused image using the
# Containerfile in the .devcontainer directory then start an ephemeral
# container with the current directory bind-mounted as "/workspace".
#
# Requires a local installation of either Docker or podman.

# Attempt to build a unique name for the OCI image based on easily
# gathered metadata; defaults to the current wordking directory's name
CONTAINER_NAME="$(basename "$(pwd)")"
# If we're inside a repo, make the OCI image name more easily
# discernible.
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    GIT_REPO_NAME="$(basename -s .git $(git config --get remote.origin.url) 2>/dev/null)"
    GIT_BRANCH_NAME="$(git rev-parse --abbrev-ref HEAD)"
    # Replace / in branch name with something path-friendly
    GIT_BRANCH_NAME="${GIT_BRANCH_NAME//\//_}"
    # Override default
    CONTAINER_NAME="${GIT_REPO_NAME}--${GIT_BRANCH_NAME}"
fi

# Make it obvious that the OCI image is for devcontainer usage
IMAGE_NAME="devc--${CONTAINER_NAME}"

# Which command to use to run OCI commands; defaults to podman, but
# will switch to Docker if podman is unavailable.
OCI_CLI="podman"
if ! which "${OCI_CLI}" >/dev/null 2>&1; then
	OCI_CLI="docker"
fi

# Build container using the recipe
"${OCI_CLI}" build -t "${IMAGE_NAME}" -f .devcontainer/Containerfile . || exit

# Run the dev container with current directory bind-mounted as "/workspace"
"${OCI_CLI}" run --rm --replace -it \
             --name "${CONTAINER_NAME}" \
             -v "$(pwd)":"/workspace":Z \
             "${IMAGE_NAME}"
