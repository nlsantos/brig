/*
   trill: a lightweight wrapper for Podman/Docker REST API calls
   Copyright (C) 2025  Neil Santos

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.
*/

// Package trill houses a thin wrapper for communicating with podman
// and Docker via their REST API.
package trill

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"sync"
	"syscall"

	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	mobyclient "github.com/moby/moby/client"
	"github.com/nlsantos/brig/writ"
	"golang.org/x/term"
)

// StartDevcontainerContainer starts and attaches to a container based
// on configuration from devcontainer.json.
//
// Requires metadata parsed from a devcontainer.json config, the
// tag/image name for the OCI image to use as base, and a name for the
// created container.
func (c *Client) StartDevcontainerContainer(p *writ.Parser, imageTag string, containerName string) error {
	slog.Debug("attempting to start and attach to devcontainer", "tag", imageTag, "name", containerName)
	containerCfg := c.buildContainerConfig(p, imageTag)
	hostCfg := c.buildHostConfig(p)

	if err := c.bindAppPorts(p, containerCfg, hostCfg); err != nil {
		slog.Error("encountered an error binding appPorts items", "error", err)
		return err
	}

	return c.StartContainer(p, containerCfg, hostCfg, containerName)
}

// StartContainer starts an existing container and attaches the
// current terminal to it to enable its usage.
func (c *Client) StartContainer(p *writ.Parser, containerCfg *container.Config, hostCfg *container.HostConfig, containerName string) error {
	if err := c.bindForwardPorts(p, containerCfg, hostCfg); err != nil {
		slog.Error("encountered an error binding forwardPorts items", "error", err)
		return err
	}
	c.bindMounts(p, hostCfg)

	slog.Debug("using container config", "config", containerCfg)
	slog.Debug("using host config", "config", hostCfg)

	ctx := context.Background()
	createResp, err := c.mobyClient.ContainerCreate(ctx, mobyclient.ContainerCreateOptions{
		Config:     containerCfg,
		HostConfig: hostCfg,
		Name:       containerName,
	})
	if err != nil {
		slog.Error("encountered an error creating a container", "error", err)
		return err
	}
	c.ContainerID = createResp.ID
	slog.Debug("container created successfully", "id", c.ContainerID)

	// Attaching to a container before it even starts is a way to get
	// around possibly missing a log replay upon attachment. A symptom
	// of that is needing to input something after the container is
	// attached to, to get, say, the shell prompt to appear.
	slog.Debug("attempting to attach to container", "id", c.ContainerID)
	attachResp, err := c.mobyClient.ContainerAttach(ctx, c.ContainerID, mobyclient.ContainerAttachOptions{
		Logs:   true,
		Stderr: true,
		Stdin:  true,
		Stdout: true,
		Stream: true,
	})
	if err != nil {
		slog.Error("encountered an error attaching to the container", "error", err)
		return err
	}
	slog.Debug("successfully attached to container", "id", c.ContainerID)
	defer attachResp.Close()

	restoreTerm, err := c.switchTerminalToRaw()
	if err != nil {
		return err
	}
	defer restoreTerm()

	waitFunc := c.attachHostTerminalToContainer(&attachResp)

	slog.Debug("attempting to start container", "id", c.ContainerID)
	// TODO: Support the container initialization options/operations
	// exposed by the devcontainer spec
	if _, err := c.mobyClient.ContainerStart(ctx, c.ContainerID, mobyclient.ContainerStartOptions{}); err != nil {
		slog.Error("encountered an error while trying to start the container", "error", err)
		return err
	}

	// Note that Docker apparently doesn't like resizing containers
	// until after it's started (Podman seems to be fine with it).
	c.SetInitialContainerSize()
	slog.Debug("container started successfully", "id", c.ContainerID)

	waitFunc()
	slog.Debug("detached from container", "id", c.ContainerID)

	return nil
}

// SetInitialContainerSize sets up the height and width of the
// container's pseudo-TTY, as well a hook to ensure that future
// changes in the host terminal's dimensions are propageted to the
// container.
//
// Also, on Windows, it's apparently more reliable to get the terminal size
// from stdout, as using stdin results in an invalid handle error.
func (c *Client) SetInitialContainerSize() {
	slog.Debug("attempting to resize container's pseudo-TTY")
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		slog.Error("encountered an error trying to get ther terminal's dimensions", "error", err)
		panic(err)
	}

	c.ResizeContainer(uint(h), uint(w)) // #nosec G115
	slog.Debug("setting up hooks to handle terminal resizing")
	c.listenForTerminalResize()
}

// ResizeContainer sets the container's internal pseudo-TTY height and
// width to the passed in values.
func (c *Client) ResizeContainer(h uint, w uint) {
	if _, err := c.mobyClient.ContainerResize(context.Background(), c.ContainerID, mobyclient.ContainerResizeOptions{
		Height: h,
		Width:  w,
	}); err != nil {
		panic(err)
	}
}

// attachHostTerminalToContainer attempts to route input from the
// terminal into the container's pseudo-TTY, and redirect the
// pseudo-TTY's output to the host terminal.
//
// Uses attachResp to facilitate the rerouting.
//
// This allows usage of the container in a terminal as one would,
// e.g., a regular shell
func (c *Client) attachHostTerminalToContainer(attachResp *mobyclient.ContainerAttachResult) func() {
	slog.Debug("setting up terminal input/output")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(os.Stdout, attachResp.Reader); err != nil && err != io.EOF {
			slog.Error("encountered an error copying container output to stdout", "error", err)
		}
	}()
	go func() {
		if _, err := io.Copy(attachResp.Conn, os.Stdin); err != nil && !errors.Is(err, syscall.EPIPE) {
			slog.Error("encountered an error copying terminal input to container", "error", err)
		}
	}()

	return wg.Wait
}

// buildContainerConfig initializes and returns a Moby
// container.Config struct for later use with containers.
func (c *Client) buildContainerConfig(p *writ.Parser, tag string) *container.Config {
	slog.Debug("building the container configuration")
	containerEnvs := []string{}
	for key, val := range p.Config.ContainerEnv {
		containerEnvs = append(containerEnvs, fmt.Sprintf("%s=%s", key, val))
	}

	containerCfg := container.Config{
		Env:          containerEnvs,
		ExposedPorts: make(network.PortSet),
		Image:        tag,
		OpenStdin:    true,
		Tty:          true,
		WorkingDir:   *p.Config.WorkspaceFolder,
	}

	if p.Config.ContainerUser != nil {
		containerCfg.User = *p.Config.ContainerUser
	}

	return &containerCfg
}

// buildHostConfig initializes and returns a Moby container.HostConfig
// struct for later use with containers.
func (c *Client) buildHostConfig(p *writ.Parser) *container.HostConfig {
	hostCfg := container.HostConfig{
		AutoRemove: true,
		Binds: []string{
			// By default, the context is mounted as the workspace folder
			fmt.Sprintf("%s:%s", *p.Config.Context, *p.Config.WorkspaceFolder),
		},
		CapAdd:       p.Config.CapAdd,
		PortBindings: make(network.PortMap),
		Privileged:   *p.Config.Privileged,
	}

	if c.MakeMeRoot {
		hostCfg.UsernsMode = "keep-id:uid=0,gid=0"
	}

	return &hostCfg
}

// bindAppPorts sets up the struct fields necessary to bind the ports
// in appPorts on the host machine.
//
// Requires containerCfg and hostCfg to be pointers to their
// respective structs.
//
// TODO: Enhance this as this is very simplistic and will break in a
// multi-container (i.e., Compose) environment
func (c *Client) bindAppPorts(p *writ.Parser, containerCfg *container.Config, hostCfg *container.HostConfig) error {
	if p.Config.AppPort != nil && len(*p.Config.AppPort) > 0 {
		exposedPorts, portMap, err := nat.ParsePortSpecs(*p.Config.AppPort)
		if err != nil {
			slog.Error("error parsing appPort", "appPort", *p.Config.AppPort, "error", err)
			return err
		}

		for port, set := range exposedPorts {
			nativePort := network.MustParsePort(port.Port())
			if nativePort.Num() < 1024 {
				unprivilegedPort, ok := network.PortFrom(c.PrivilegedPortElevator(nativePort.Num()), nativePort.Proto())
				if !ok {
					return fmt.Errorf("could not convert privileged port into an unprivileged one: %#v", nativePort)
				}
				containerCfg.ExposedPorts[unprivilegedPort] = set
			}
			containerCfg.ExposedPorts[network.MustParsePort(port.Port())] = set
		}

		for port, bindings := range portMap {
			var portBindings []network.PortBinding
			for _, binding := range bindings {
				hostIP := binding.HostIP
				if len(hostIP) == 0 {
					// Maybe make this configurable so ports can be exposed to beyond localhost?
					hostIP = "127.0.0.1"
				}

				hostPort := network.MustParsePort(binding.HostPort)
				if hostPort.Num() < 1024 {
					unprivilegedPort, ok := network.PortFrom(c.PrivilegedPortElevator(hostPort.Num()), hostPort.Proto())
					if !ok {
						return fmt.Errorf("could not convert privileged appPorts into an unprivileged one: %#v", hostPort)
					}
					slog.Debug("converted a privileged appPorts to an unprivileged one", "old-port", hostPort.Num(), "new-port", unprivilegedPort.Num())
					binding.HostPort = strconv.Itoa(int(unprivilegedPort.Num()))
				}

				portBindings = append(portBindings, network.PortBinding{
					HostIP:   netip.MustParseAddr(hostIP),
					HostPort: binding.HostPort,
				})
			}
			hostCfg.PortBindings[network.MustParsePort(port.Port())] = portBindings
		}
	}

	return nil
}

// bindForwardPorts sets up the struct fields necessary to bind the
// ports in forwardPorts on the host machine.
//
// Requires containerCfg and hostCfg to be pointers to their
// respective structs.
//
// TODO: Add a brig option to specify that ports in forwardPort
// should listen on 0.0.0.0 instead of 127.0.0.1
func (c *Client) bindForwardPorts(p *writ.Parser, containerCfg *container.Config, hostCfg *container.HostConfig) error {
	if len(p.Config.ForwardPorts) < 1 {
		return nil
	}

	for _, forwardPort := range p.Config.ForwardPorts {
		port, err := network.ParsePort(forwardPort)
		if err != nil {
			slog.Error("cannot parse forward port", "port", forwardPort, "error", err)
			return err
		}
		containerCfg.ExposedPorts[port] = struct{}{}
		portNum, err := strconv.Atoi(forwardPort)
		if err != nil {
			return err
		}
		if portNum < 1023 {
			unprivilegedPort, ok := network.PortFrom(c.PrivilegedPortElevator(uint16(portNum)), network.TCP)
			if !ok {
				return fmt.Errorf("could not convert privileged forwardPorts into an unprivileged one: %#v", portNum)
			}
			slog.Debug("converted a privileged forwardPorts to an unprivileged one", "old-port", portNum, "new-port", unprivilegedPort.Num())
			forwardPort = strconv.Itoa(int(unprivilegedPort.Num()))

		}
		hostCfg.PortBindings[port] = []network.PortBinding{
			{
				HostIP:   netip.MustParseAddr("127.0.0.1"),
				HostPort: forwardPort,
			},
		}
	}

	return nil
}

// bindMounts sets up bind and/or volume mounts.
//
// Requires hostCfg to its respective struct.
func (c *Client) bindMounts(p *writ.Parser, hostCfg *container.HostConfig) {
	if len(p.Config.Mounts) > 0 {
		var mounts = []mount.Mount{}
		for _, mountEntry := range p.Config.Mounts {
			mountItem := mount.Mount{
				Source: mountEntry.Mount.Source,
				Target: mountEntry.Mount.Target,
			}
			switch mountEntry.Mount.Type {
			case "bind":
				mountItem.Type = mount.TypeBind
			case "volume":
				mountItem.Type = mount.TypeVolume
			}
			mounts = append(mounts, mountItem)
		}
		hostCfg.Mounts = mounts
	}
}

// switchTerminalToRaw attempts to switch the current terminal to raw
// mode.
//
// If no errors are encountered, returns a function that restores the
// previous state of the terminal.
//
// Switching the terminal to raw mode ensures that input with
// control characters (e.g., Ctrl-D) get passed through to the
// container
func (c *Client) switchTerminalToRaw() (func(), error) {
	slog.Debug("switching terminal to raw mode")
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("%#v is not a terminal", fd)
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		slog.Error("encountered an error while trying to switch terminal to raw mode", "error", err)
		return nil, err
	}

	return func() {
		slog.Debug("restoring terminal state")
		if err := term.Restore(fd, oldState); err != nil {
			slog.Error("encountered an error while trying to restore terminal state", "error", err)
			panic(err)
		}
	}, nil
}
