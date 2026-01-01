# brig

[![Go Report Card](https://goreportcard.com/badge/github.com/nlsantos/brig)](https://goreportcard.com/report/github.com/nlsantos/brig)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![GitHub release](https://img.shields.io/github/release/nlsantos/brig.svg)](https://github.com/nlsantos/brig/releases)
![Commits since last release](https://img.shields.io/github/commits-since/nlsantos/brig/latest)

**The lightweight, native Go CLI for devcontainers**

`brig` reads your `devcontainer.json` configuration and spins up a containerized development environment. It is designed as a standalone, dependency-free alternative to the [official command-line tool](https://github.com/devcontainers/cli) with first-class support for [podman](https://podman.io/) and rootless workflows.

![`brig` running its own devcontainer](.github/assets/brig.gif)

## Table of contents

- [Prerequisites](#prerequisites)
- [Quick start](#quick-start)
- [Why use `brig`?](#why-use-brig)
- [Podman-first design](#podman-first-design)
- [Features](#features)
- [Alternatives](#alternatives)
- [Incompatibilities](#incompatibilities)

## Prerequisites

Before installing `brig`, ensure you have the following:

- **OCI container runtime that implements Docker's REST API:** A running instance of [podman](https://podman.io/) (highly recommended) or Docker.
- **An accessible networking socket:** See [instructions for enabling podman's socket on *nix](https://github.com/containers/podman/blob/main/docs/tutorials/socket_activation.md).

## Quick start

### Via Go

```bash
go install github.com/nlsantos/brig/cmd/brig@latest
```

### Via Homebrew

```bash
brew install nlsantos/tap/brig
```

### Manual install

Download the [latest release](releases) for your platform and extract the binary to a directory in your `$PATH` (e.g., `~/.local/bin`).

### Usage

- `cd` into a directory with a `devcontainer.json`.

- Run `brig`.

- Wait for the build to complete. Once finished, your terminal will be attached to the devcontainer.

> ⚠️ Note on persistence: `brig` treats containers as ephemeral. When you exit the shell, the container is removed. Ensure all persistent work is saved in your project directory (which is mounted) or defined in the `devcontainer.json` configuration.

### Options

- **Help**: Run `brig --help` to see all supported flags.
- **Configuration**: `brig` looks for a `brigrc` configuration file in `${HOME}/.config/brigrc`, `${HOME}/.brigrc`, or `${USERPROFILE}/.brigrc`. See [brigrc](brigrc) for a sample.

## Why use `brig`?

1. **Lightweight & Fast**: Unlike the official command-line tool, `brig` is a single static binary. It installs instantly, starts up immediately, and requires no massive dependency tree.
2. **Minimalist Design**: Built along the lines of the Unix philosophy of building one thing that does one thing well, `brig` strives to do its job and get out of your way.
3. **Editor Agnostic**: `brig` unlocks the powerful and convenient workflow enabled by devcontainers to users of Emacs, Vim, Helix, and other editors.
4. **Security Focus**: Built with [podman](https://podman.io/) in mind, `brig`'s implementation choices are made in alignment with podman's design of running containers as a regular user. This aligns well with usage in highly locked-down environments (e.g., company-issued workstations).
5. **FreeBSD Support**: With podman being available in FreeBSD, users who prefer a *nix-based operating system have another choice beyond GNU/Linux and macOS. `brig` can help maintain a similar workflow to those using Windows and Visual Studio Code.

## Podman-first design

The devcontainer spec is written primarily with the assumption that the underlying container platform is Docker. `brig` was built to treat [podman](https://podman.io/) as a first-class citizen.

I prefer podman for its rootless design. While `brig` uses Moby's packages and the Docker REST API, development prioritizes compatibility with podman. If the Moby packages ever become incompatible with Podman, `brig` will remain on the latest version that is.

_To summarize, Docker support is achieved via Podman's compatibility with the Docker REST API and Moby packages. While `brig` works seamlessly with Docker, feature development prioritizes Podman's rootless architecture._

## Features

While `brig` is currently in **alpha**, it supports the core devcontainer workflow:

- **Spec compliance:** Validates `devcontainer.json` configuration against the official schema.
- **Container lifecycle:** Builds images (via `dockerFile`) or pull images from remote registries (via `image`) and creates containers, using Git metadata when possible.
- **Container configuration:** Supports `capAdd`, `privileged` mode, `mounts`, `containerEnv`.
- **Networking:** Binds ports specified in `appPorts` and `forwardPorts`.
- **Variable expansion:** Robust variable expansion inspired by standard Unix shells powered by [mvdan/sh](https://github.com/mvdan/sh).

_For a more expansive list of features, refer to [docs/features.md](docs/features.md)._

## Alternatives

- vs. **[devcontainers/cli](https://github.com/devcontainers/cli)**: As the reference implementation, the official command-line tool implements all the features of the spec, but requires the Node.js runtime. `brig` is a compiled Go binary, making it faster to deploy and simpler to manage.
- vs. **[UPwith-me/Container-Maker](https://github.com/UPwith-me/Container-Maker)**: `cm` implements features that are tangential to the core devcontainer workflow; while the bells and whistles are nice (and very impressive), I prefer a tool more aligned to the Unix philosophy.

## Incompatibilities

These are the known differences with the observed behavior of Visual Studio Code and/or the official devcontainer command-line tool.

### Port Management & Networking

`brig` differs from the official spec regarding port forwarding and privilege elevation to strictly adhere to rootless security principles.

- **No privilege elevation:** `brig` will not attempt to gain elevated privileges to bind low-numbered ports.
- **Privileged ports remapping:** Instead of privilege elevation, `brig` offsets the port number on host side by a preset figure (defaults to `8000` but can be set via the `-p` or `--port-offset` flags).
- **`appPort` vs `forwardPorts`:** `brig` prefers `appPort` for predictable host mapping.

For a detailed technical explanation of these design choices, see [docs/ports.md](docs/ports.md).

### Ephemeral containers

`brig` treats devcontainers as ephemeral, unlike Visual Studio Code (and possibly the official command-line tool), which keeps stopped containers to start later.

This aligns with the "cattle, not pets" philosophy for development environments, and encourages devcontainers to be stateless and reproducible.

### No dedicated build step

Changes to `devcontainer.json` take effect immediately on the next run. There is no separate "Rebuild Container" step required; just run `brig` again.

### No runArgs support

The `runArgs` field (arbitrary Docker CLI flags) is not supported because `brig` interacts with the engine via the REST API. Direct API equivalents (where applicable) are implemented via specific fields (like `capAdd`) instead.

---

_Originally written because I'm an Emacs and podman user and don't want to have to deal with Node.js._
