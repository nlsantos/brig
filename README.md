# brig, a tool for working in devcontainers

This tool reads your `devcontainer.json` file then brings up a container for you to work in based on that configuration.

`brig` attempts to conform to the [published devcontainer.json
specs](https://github.com/devcontainers/spec); it's a long ways from it, but the goal is to eventually be an alternative to the [official command-line tool](https://github.com/devcontainers/cli).

`brig` also treats [podman](https://podman.io/) as a first-class citizen, since I prefer it over Docker.

## Why

[devcontainers](https://containers.dev) are pretty nifty. It's also **very** nifty that, _technically_, they're not tied to Visual Studio Code. This suits me, as I consider myself an Emacs user.

For the most part, one can make do with relatively simple shell scripts to build a container based on the `Containerfile`/`Dockerfile` that usually accompanies a devcontainer recipe. (This what I did for several years before writing brig; see [`start-dev-container.sh`](https://github.com/nlsantos/brig/blob/38d4ae10557422c37af349c9df3b460c343d487c/start-dev-container.sh))

However, as I've been diving into the spec, I've been finding more and more little things that I've been wanting to implement in the devcontainer recipes my team is using (e.g., lifecycle commands).

While those can be replicated (for the most part) using the aforementioned shell scripts, the information is already there in the devcontainer recipe: it seems a shame to have that information be duplicated when it need not be.

**I also wanted to learn Go.** I've been cosplaying as a DevOps
engineer for the past ~10 years and most of the programming I've done have been little helper scripts in either shell or Python. I figured it was high time I got back into working on free software.

## Why not...?

- **[devcontainers/cli](https://github.com/devcontainers/cli)**
  - The official command-line tool is a Node app; 'nuff said.
- **[UPwith-me/Container-Maker](https://github.com/UPwith-me/Container-Maker)**
  - `cm` is pretty much built around Docker; it _might_ be possible to use it with podman, but I found out about the project after I've already spent a couple of days writing `brig`
  - I do find their [entrypoint script](https://github.com/UPwith-me/Container-Maker/blob/main/pkg/runner/entrypoint.go) pretty interesting, so I'll probably ~~steal~~ adopt that at some point

## What works

Keep in mind that `brig` is still very much pre-alpha software at this point. While as of [38d4ae1](https://github.com/nlsantos/brig/commit/38d4ae10557422c37af349c9df3b460c343d487c), `brig` is being developed inside a devcontainer ran by itself, it's still missing support for a lot of fields.

That said, here's a list of what `brig` can do:

- [x] Automatically finding `devcontainer.json` files as per the spec
  - [x] Supports specifying a path for a `devcontainer.json` through a command-line parameter, if your config doesn't conform to the usual naming
- [x] Validation of the target `devcontainer.json` file against the official spec
- [x] Building an image based on the `dockerFile` field, targeting the value of the `context` field
- [x] Creating a container based on the image it builds
- [x] Attaching the terminal to the container
  - [x] Resizing the internal pseudo-TTY of the container dynamically based on your terminal's reported dimensions

## Incompatibilities

These are the known incompatibilities with the way Visual Studio Code and/or the official devcontainer command line tool works. They _may_ change in the future, depending on patches or changes in my preferred workflow.

### Ephemeral containers

I like my devcontainers ephemeral and pretty much stateless. On the other hand, Visual Studio Code (and possibly the official command line tool) keeps around stopped containers and just restarts them on subsequent usage.

The spec, to my knowledge, doesn't mandate that stopped containers be kept around; it's _implied_ (see the values for the `shutdownAction` field), but I haven't (yet?) come across anything in the way devcontainers work that would necessitate keeping stopped containers around, so I just wrote `brig` to conform to my preferences.
