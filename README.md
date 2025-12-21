# brig, the lightweight, native Go CLI for devcontainers
This tool reads your `devcontainer.json` file then brings up a container for you to work in based on that configuration.

`brig` attempts to conform to the [published devcontainer.json
specs](https://github.com/devcontainers/spec); it's a long ways from it, but the goal is to eventually be an alternative to the [official command-line tool](https://github.com/devcontainers/cli).

`brig` also treats [podman](https://podman.io/) as a first-class citizen, since I prefer it over Docker.

## Table of contents

- [Quick start](#quick-start)
- [Why](#why)
- [What does "podman as a first-class citizen" mean?](#what-does-podman-as-a-first-class-citizen-mean)
- [Why not...?](#why-not)
- [What works](#what-works)
- [Incompatibilities](#incompatibilities)

## Quick start

- Download the latest [release](releases) for your platform then extract the binary to somewhere available in your `$PATH` (maybe `~/.local/bin`?).

- Alternatively, install `brig` using `go`:

```bash
go install github.com/nlsantos/brig/cmd/brig@latest
```

- Using your preferred terminal, `cd` into a directory with a `devcontainer.json`.

- Run `brig`, wait for the build process to complete, and you should be plopped into a shell inside your devcontainer.

### Options

`brig` supports a handful of parameters that influence the way it works. To view the list, run:

```bash
brig --help
```

### Flags file

Refer to [`brigrc`](brigrc) for a sample configuration file. `brig` will find it automatically if you place it in either `"${HOME}/.config/brigrc"` (e.g., on \*nix) or `"${USERPROFILE}/.brigrc"` (e.g., on Windows).

### Docker users

To get `brig` working with Docker, you need to specify the socket address (or named pipe on Windows) Docker listens on by passing it to the `--socket` command line argument or the `"socket ="` entry in the flags file.

(To save you a search, on Windows plus Docker, the value is likely to be `npipe:////./pipe/docker_engine`)

## Why

[devcontainers](https://containers.dev) are pretty nifty. It's also **very** nifty that, _technically_, they're not tied to Visual Studio Code. This suits me, as I consider myself an Emacs user.

For the most part, one can make do with relatively simple shell scripts to build a container based on the `Containerfile`/`Dockerfile` that usually accompanies a devcontainer recipe. (This what I did for several years before writing brig; see [`start-dev-container.sh`](blob/38d4ae10557422c37af349c9df3b460c343d487c/start-dev-container.sh))

However, as I've been diving into the spec, I've been finding more and more little things that I've been wanting to implement in the devcontainer recipes my team is using (e.g., lifecycle commands).

While those can be replicated (for the most part) using the aforementioned shell scripts, the information is already there in the devcontainer recipe: it seems a shame to have that information be duplicated when it need not be.

**I also wanted to learn Go.** I've been cosplaying as a DevOps
engineer for the past ~10 years and most of the programming I've done have been little helper scripts in either shell or Python. I figured it was high time I got back into working on free software.

## What does "podman as a first-class citizen" mean?

The devcontainer spec is written primarily with the assumption that the underlying container platform is Docker. The naming of the fields (`dockerFile`, `dockerComposeFile`) attests to that.

I only use Docker when I'm forced to; I prefer Podman primarily due to its rootless design. While `brig` uses Moby's packages, and I'm referencing the Docker REST API primarily, it's only ever going to be to the extent that Podman is able to stay compatible with it.

Should the Moby packages ever become incompatible with Podman, `brig` will remain with the latest version that is. I will also never implement or integrate a devcontainer feature or capability that Podman does not support.

Basically, the fact that `brig` works with Docker is a _side-effect_ of Podman's compatibility with the Docker REST API and Moby packages. I will make changes that promote feature parity, but if support for Docker becomes too onerous to maintain, I won't hesitate to drop it.

## Why not...?

- **[devcontainers/cli](https://github.com/devcontainers/cli)**
  - The official command-line tool is a Node app; 'nuff said.
- **[UPwith-me/Container-Maker](https://github.com/UPwith-me/Container-Maker)**
  - ~~`cm` is pretty much built around Docker; it _might_ be possible to use it with podman,~~ ([`cm` supports Podman](https://github.com/UPwith-me/Container-Maker?tab=readme-ov-file#rootless-support)) but I found out about the project after I've already spent a couple of days writing `brig`
  - I do find their [entrypoint script](https://github.com/UPwith-me/Container-Maker/blob/main/pkg/runner/entrypoint.go) pretty interesting, so I'll probably ~~steal~~ adopt that at some point

## What works

Keep in mind that `brig` is still very much alpha software at this point. While as of [38d4ae1](commit/38d4ae10557422c37af349c9df3b460c343d487c), `brig` is being developed inside a devcontainer ran by itself, it's still missing support for a lot of fields.

That said, here's a list of what `brig` can do:

### Operations

- [x] Automatically finding `devcontainer.json` files as per the spec
- [x] Supports specifying a path for a `devcontainer.json` through a command-line parameter, if your config doesn't conform to the expected naming
- [x] Specifying the socket address to use to connect to the container daemon, in case `brig` can't find it automatically

### Parsing and validation

- [x] Validation of the target `devcontainer.json` file against the official spec

### Basic container operations

- [x] Pulling the image specified `image` field from remote registries
- [x] Building an image based on the `dockerFile` field, targeting the value of the `context` field
- [x] Creating a container based on the image it builds
- [x] Attaching the terminal to the container
  - [x] Resizing the internal pseudo-TTY of the container dynamically based on your terminal's reported dimensions
- [x] (_Very_) basic support for forwarding ports; see additional notes [re: privilege elevation](#elevation-for-port-bindings) and [re: forwarding methods](appport-vs-forwardports).

### devcontainer-specific features

- [x] Specifying a different UID to use inside the devcontainer via the ~~`remoteUser`~~ `containerUser` field (fixed as of [ed8e31b](commit/ed8e31ba4023eab3ab618675757b833e2425c978))
- [x] Specifying kernel capabilities to add to the container via the `capAdd` field
- [x] Specifying that the container should run in privileged mode via the `privileged` field
- [x] Special environment variables (`containerWorkspaceFolder`, `localEnv`, etc.) work!
  - Okay, they _partially_ work: `${containerEnv:*}` is a work in progress
- [x] Variable expansion (e.g., `${env:UNDEF_VAR:-default}` returns "default" if `UNDEF_VAR` doesn't exist)
- [x] Mounting volumes as specified by the `mounts` field
  - [x] Using variables and variable expansion in `mounts` items work as expected

### Useful extras

Variable expansions go a little farther than what's available in the devcontainer spec: You can even do some other shell-inspired things with them, as long as they're supported by the [mvdan.cc/sh/v3](https://github.com/mvdan/sh) package. For examples of what operations are supported, refer to [writ/writ_test.go](writ/writ_test.go).

Check out [Bash's Shell Parameter Expansion](https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html) to get an idea of what you can do. Just be aware that not all of them will be supported, or even make sense in this context.

That said, _being able to_ doesn't mean you _should_, as relying on features outside of the devcontainer spec will necessarily mean you'll be sacrificing interoperability.

## Incompatibilities

These are the known incompatibilities with the way Visual Studio Code and/or the official devcontainer command-line tool works. They _may_ change in the future, depending on patches or changes in my preferred workflow.

### Ephemeral containers

I like my devcontainers ephemeral and pretty much stateless. On the other hand, Visual Studio Code (and possibly the official command-line tool) keeps around stopped containers and just restarts them on subsequent usage.

The spec, to my knowledge, doesn't mandate that stopped containers be kept around; it's _implied_ (see the values for the `shutdownAction` field), but I haven't (yet?) come across anything in the way devcontainers work that would necessitate keeping stopped containers around, so I just wrote `brig` to conform to my preferences.

### No dedicated build step

Related to the previous point. I also prefer that changes to the devcontainers take effect immediately the next time I open it, as opposed to having to explicitly initiate a rebuild.

I realize that larger codebases would likely benefit from a build-only step, as well as persistent containers: a <100MB codebase I work with that pulls a couple of >1GB images during building takes about 16 seconds from running `brig` to the prompt being ready, and that's with the image already having been built.

However, I've found in practice that it's not necessarily an issue, as I spin up the devcontainer in a terminal and immediately switch back to Emacs to resume working.

For what it's worth, I'm not _opposed_ to having a dedicated build step; I'm just not convinced of its necessity, and I'm wary of what I perceive would be a penalty to my workflow.

### Elevation for port bindings

Never going to be officially supported. I've got an idea of how `brig` will handle configuration that specifies a privileged port, but it will not involve privilege elevation.

### appPort vs forwardPorts

The spec [recommends using `forwardPorts` over `appPort`](https://containers.dev/implementors/json_reference/#:~:text=In%20most%20cases%2C%20we%20recommend%20using%20the%20new%20forwardPorts%20property%2E), but it seems to me that the latter is inferior:

- `forwardPorts` only supports TCP connections:
  - The `protocol` field in `portAttributes` and `otherPortsAttributes` only allows either `http` or `https`
  - When `protocol` is unset, implementing tools are supposed to act as though it's set to `tcp`
  - The spec will flag your configuration as invalid if you specify `tcp` (or anything else) explicitly.
- `forwardPorts` does not support explicitly mapping container ports to a different port on the host:
  - If `RequireLocalPort` on its corresponding `portAttributes` entry (or in `otherPortsAttributes`) is set to `false` (the default), the implementing tool is expected to silently map it to an arbitrary port.

The spec also has [a blurb regarding publishing vs. forwarding ports](https://containers.dev/implementors/json_reference/#publishing-vs-forwarding-ports) that seems to imply that the official tool treats `appPort` entries as container-only ports (i.e., not accessible on the host machine) by default.

I'm yet to check and verify this behavior using Visual Studio Code, and will update this section when I do.

Note that `brig` will expose on the host machine all ports (if inputted correctly) specified in `appPort`.

### No runArgs support

The devcontainer spec defines a field named `runArgs`. It's an array of command line parameters to pass to Docker when running the container. I think the intent is to allow passing args that aren't covered by other fields.

It's commendable: it ensures that the spec doesn't have to cover _all_ use-cases, and permits integrating devcontainer usage in workflows the spec authors can't even imagine.

However, owing to the fact that `brig` communicates with Podman and Docker via their REST API, I don't see a way of directly supporting `runArgs` aside from parsing each entry and trying to translate parameters into their API equivalents (if any).

All this to say, I don't see `brig` ever supporting `runArgs`, barring a major architectural change.
