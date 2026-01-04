---
layout: page
title: Features
sidebar_link: true
---
Keep in mind that `brig` is still very much alpha software at this point. While as of [38d4ae1](../commit/38d4ae10557422c37af349c9df3b460c343d487c), `brig` is being developed inside a devcontainer ran by itself, it's still missing support for a lot of fields.

That said, here's a list of what `brig` can do:

## Operations

- [x] Automatically finding `devcontainer.json` files as per the spec
- [x] Supports specifying a path for a `devcontainer.json` through a command-line parameter, if your config doesn't conform to the expected naming
- [x] Specifying the socket address to use to connect to the container daemon, in case `brig` can't find it automatically

## Parsing and validation

- [x] Validation of the target `devcontainer.json` file against the official spec

## Basic container operations

- [x] Pulling the image specified `image` field from remote registries
- [x] Building an image based on the `dockerFile` field, targeting the value of the `context` field
- [x] Creating a container based on the image it builds
- [x] Attaching the terminal to the container
  - [x] Resizing the internal pseudo-TTY of the container dynamically based on your terminal's reported dimensions
- [x] (_Very_) basic support for forwarding ports; see additional notes [re: privilege elevation](../#port-management--networking) and [re: forwarding methods](ports.md).

## Composer projects
- [x] Support for spinning up Composer projects
- [x] Building containers via either Containerfiles/Dockerfiles or `image`.
- [x] Automatic teardown of Composer projects upon exiting from the devcontainer

## devcontainer-specific features

- [x] Specifying a specific user/UID to use inside the devcontainer via the `containerUser` field
- [x] Support for [lifecycle scripts](https://containers.dev/implementors/json_reference/#lifecycle-scripts), including the ability to specify which user they should run as via the `remoteUser` field
- [x] Specifying kernel capabilities to add to the container via the `capAdd` field
- [x] Specifying that the container should run in privileged mode via the `privileged` field
- [x] Special environment variables (`containerWorkspaceFolder`, `localEnv`, etc.) work!
  - Okay, they _partially_ work: `${containerEnv:*}` is a work in progress
- [x] Variable expansion (e.g., `${env:UNDEF_VAR:-default}` returns "default" if `UNDEF_VAR` doesn't exist)
- [x] Mounting volumes as specified by the `mounts` field
  - [x] Using variables and variable expansion in `mounts` items work as expected

## Useful extras

Variable expansions go a little farther than what's available in the devcontainer spec: You can even do some other shell-inspired things with them, as long as they're supported by the [mvdan.cc/sh/v3](https://github.com/mvdan/sh) package. For examples of what operations are supported, refer to [writ/writ_test.go](../writ/writ_test.go).

Check out [Bash's Shell Parameter Expansion](https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html) to get an idea of what you can do. Just be aware that not all of them will be supported, or even make sense in this context.

That said, _being able to_ doesn't mean you _should_, as relying on features outside of the devcontainer spec will necessarily mean you'll be sacrificing interoperability.
