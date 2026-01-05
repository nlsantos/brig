---
layout: page
title: Features
sidebar_link: true
---
> 🐶 Since [`38d4ae1`](https://github.com/nlsantos/brig/commit/38d4ae10557422c37af349c9df3b460c343d487c), `brig` has been developed inside a devcontainer managed by itself.

While it is still  **alpha software**, `brig` supports enough features of the spec that make it a viable tool for most common uses of devcontainers.

| Category | Feature | Status | Notes |
| :------- | :------ | :----: | :---- |
| **Core operations** | **Configuration discovery** | ✅️ | Auto-discovers if config is in [one of the paths noted by the spec](https://containers.dev/implementors/spec/#devcontainerjson); can also specify a target path via flags |
| | **Validation** | ✅️ | Validates config against [the official spec](https://raw.githubusercontent.com/devcontainers/spec/d424cc157e9a110f3bf67d311b46c7306d5a465d/schemas/devContainer.base.schema.json) |
| **Container configuration** | **`capAdd`** | ✅️ | Fully supported |
| | **`privileged`** | ✅️ | Fully supported [with caveats](#privileged-mode) |
| | **[Host requirements](https://containers.dev/implementors/json_reference/#min-host-reqs)** | ❓️ | Planned, but low priority |
| **Environment variables** | **[Special variables](https://containers.dev/implementors/json_reference/#variables-in-devcontainerjson)** | ✅️️️ | Fully supported |
| | **Variable expansion** | ✅️️️ | Fully supported, with [extra features](#variable-expansion) |
| **Lifecycle** | **Image-based** | ✅️ | Pulls from remote registries |
| | **Build-based** | ⚠️️ | Builds via `dockerFile` using `context`; support for `build.*` fields is a WIP |
| | **Composer project** | ⚠️️️ | Multiple services via `dockerComposeFile`; support for `runServices` is a WIP |
| | **[Lifecycle scripts](https://containers.dev/implementors/json_reference/#lifecycle-scripts)** | ✅️ | Supports `initializeCommand`, `postCreateCommand`, etc. and running as a separate user via `remoteUser` |
| | **`runArgs`** | ❓️ | Planned, but low priority |
| **Exposing services** | **Port forwarding** | ✅️ | Supports `appPorts` and `forwardPorts` without needing admin rights; see [ports management](ports.md) |
| **File/volume management** | **`mounts` field** | ✅️ | Fully supported (including variable expansion) |
| | **File ownership** | ⚠️ | For containers where the user is `root`, ownership **Just Works**; support for containers that use a non-`root` user internally is a WIP |
| **Workflow** | **Terminal attachment** | ✅️ | Automatically attaches your terminal to the devcontainer once it's ready |
| | **Cleanup** | ✅️ | Automatically tears down containers upon the devcontainer's exit |

## No elaborate pre-setup rituals

> **Just `cd` into your project's directory and run `brig`.**

That's it: `brig` doesn't need a separate configuration file if its defaults suffice for your needs.

- **No monitoring image builds just to know when to run an attach command.** _`brig` will attach your terminal to the devcontainer automatically._
- **No need to run a different command to run something in the container.** _Once your terminal is attached, just run commands as you normally would._
- **No need to run _yet another command_ to clean up.** _`brig` will clean up after your devcontainer exits._

## Security-minded

- **Local-only binding:** By default, `brig` binds ports to `127.0.0.1`. Your development services remain accessible to you, but hidden from the local network.
- **No `root` required:** `brig` **does not** use privilege escalation to bind low-numbered ports. Instead, it *offsets* them. See [docs/ports.md](ports.md) for details.
- **Offline capable:** `brig` makes no network calls other than to the OCI runtime's REST API. If your images are pre-downloaded, you can build and run devcontainers entirely offline.

## Keep things readable

Instead of generic container IDs, `brig` will try to use metadata from your project to generate names that make sense at a glance.

For example, my `brig` worktree in `~/brig/main` has a top-level devcontainer, while the `docs` directory has its own that uses a Composer project.

```text
~/brig/main/           <-- Running 'brig' here creates the devcontainer: 'brig--main'
 └── .devcontainer/
 └── docs/             <-- Running 'brig' here creates a Composer project with two containers:
   └── .devcontainer/      'brig--main--jekyll' and 'brig--main--jekyll-serve'
```

This makes it easy to distinguish between environments especially useful when you have multiple projects with devcontainers running at the same time.

Readability extends to images built via `Containerfile`, and is very useful if you're utilizing [Git's worktrees](https://git-scm.com/docs/git-worktree).

For example, if your devcontainer uses a `Containerfile` to build a custom image, the generated image will be named `localhost/devc--<basename>--<branch>>`, making image management easier.

### Privileged mode

Unless you are running as `root` on the host, running a privileged container under [podman](https://podman.io) is not the same as running it under Docker.

_See [podman's documentation on the `--privileged` flag](https://docs.podman.io/en/v4.6.1/markdown/options/privileged.html)._

### Variable expansion

Variable expansion in `brig` go a little farther than what's available in the devcontainer spec: You can even do some other shell-inspired things with them, as long as they're supported by the [mvdan.cc/sh/v3](https://github.com/mvdan/sh) package.

For examples of what operations are supported, refer to [writ/writ_test.go](https://github.com/nlsantos/brig/blob/main/writ/writ_test.go).

Refer to [Bash's Shell Parameter Expansion](https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html) to get an idea of what you can do. Just be aware that not all of them will be supported, or even make sense in the context of devcontainer configuration.

> ⚠️ **Extended variable expansion  is not supported by the devcontainer spec.** Using it will break compatibility with Visual Studio Code and other devcontainer implementations.
