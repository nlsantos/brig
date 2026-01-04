---
layout: page
title: Port management
sidebar_link: true
---
# Port management

## Privilege elevation

The devcontainer specification expects implementations to support privilege elevation in order to support binding "privileged ports" (ports numbered lower than 1024).

`brig` **does not support** privilege elevation for port binding. This is a deliberate design choice to maintain strict compatibility with the rootless philosophy of Podman and avoid friction when devcontainers are deployed in locked-down environments (e.g., company-issued laptops).

## `appPort` vs `forwardPorts`

While the devcontainer specification [recommends using `forwardPorts` over `appPort`](https://containers.dev/implementors/json_reference/#:~:text=In%20most%20cases%2C%20we%20recommend%20using%20the%20new%20forwardPorts%20property%2E), `brig` treats `appPort` as the primary method for binding ports. This is due to several technical limitations inherent to `forwardPorts`:

1. **Protocol Limitations:** `forwardPorts` only supports TCP:

- The `protocol` field in `portAttributes` only allows `http` or `https` (which imply TCP).
- If `protocol` is unset, it's supposed to be treated as though it was set to `tcp`.
- Explicitly specifying `tcp` (or `udp`) causes a devcontainer.json to fail validation.

2. **Lack of Mapping Control:** `forwardPorts` does not support explicit host-to-container mapping (e.g., mapping container port 3000 to host port 8080).

- If `RequireLocalPort` is set to `false` (the default), implementing tools are expected to silently map the container port to an arbitrary ephemeral port on the host.
- This unpredictability breaks workflows that rely on fixed addresses (e.g., OAuth callbacks or bookmarking `localhost:8080`).

### Behavior difference

The specification has [a section discussing publishing vs. forwarding ports](https://containers.dev/implementors/json_reference/#publishing-vs-forwarding-ports) that implies the official tool treats `appPort` entries as container-only ports (not accessible on the host machine) by default.

In contrast, `brig` **will expose all ports specified in `appPort` to the host machine** (provided they are specified correctly).
