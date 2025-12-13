# brig, a tool for working with devcontainers

This tool tries reads your `devcontainer.json` file then brings up a
container for you to work in based on that configuration.

`brig` attempts to conform to the [published devcontainer.json
specs](https://github.com/devcontainers/spec); it's a long ways from
it, but the goal is to eventually be an alternative to the [official
command-line tool](https://github.com/devcontainers/cli).

`brig` also treats [podman](https://podman.io/) as a first-class
citizen, since I prefer it over Docker.

## Why

[devcontainers](https://containers.dev) are pretty nifty. It's also
**very** nifty that, _technically_, they're not tied to Visual Studio
Code. This suits me, as I consider myself an Emacs user.

For the most part, one can make do with relatively simple shell
scripts to build a container based on the `Containerfile`/`Dockerfile`
that usually accompanies a devcontainer recipe.

However, as I've been diving into the spec, I've been finding more and
more little things that I've been wanting to implement in the
devcontainer recipes my team is using (e.g., lifecycle commands).

While those can be replicated (for the most part) using the
aforementioned shell scripts, the information is already there in the
devcontainer recipe: it seems a shame to have that information be
duplicated when it need not be.

**I also wanted to learn Go.** I've been cosplaying as a DevOps
engineer for the past ~10 years and most of the programming I've done
have been little helper scripts in either shell or Python. I figured
it was high time I got back into working on free software.

## Why not...?

- **[devcontainers/cli](https://github.com/devcontainers/cli)**
  - The official command-line tool is a Node app; 'nuff said.
- **[UPwith-me/Container-Maker](https://github.com/UPwith-me/Container-Maker)**
  - `cm` is pretty much built around Docker; it _might_ be possible to
    use it with podman, but I found out about the project after I've
    already spent a couple of days writing `brig`
  - I do find their [entrypoint
    script](https://github.com/UPwith-me/Container-Maker/blob/main/pkg/runner/entrypoint.go)
    pretty interesting, so I'll probably ~~steal~~ adopt that at some
    point
