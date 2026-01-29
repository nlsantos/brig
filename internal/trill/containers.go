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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/docker/go-connections/nat"
	imagespec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	mobyclient "github.com/moby/moby/client"
	"github.com/nlsantos/brig/writ"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/term"
)

// ErrLifecycleHandler is a generic error thrown when the lifecycle
// handler encounters an error
var ErrLifecycleHandler = errors.New("lifecycle handler encountered an error")

// ExecInDevcontainer runs a command inside the designated
// devcontainer (i.e., the lone container in non-Composer
// configurations, or the one named in the service field otherwise).
func (c *Client) ExecInDevcontainer(ctx context.Context, p *writ.DevcontainerParser, runInShell bool, args ...string) error {
	return c.ExecInContainer(ctx, c.ContainerID, p, runInShell, args...)
}

// ExecInContainer runs a command inside a container designated by
// containerID.
//
// If runInShell is true, args is ran via `/bin/sh -c`; otherwise,
// args[0] is treated as the program name.
func (c *Client) ExecInContainer(ctx context.Context, containerID string, p *writ.DevcontainerParser, runInShell bool, args ...string) (err error) {
	if runInShell {
		shellCmd := []string{"/bin/sh", "-c"}
		args = append(shellCmd, args...)
	}
	cmd := strings.Join(args, " ")
	slog.Info("running command in container", "container", containerID, "cmd", cmd)

	execCreateOpts := mobyclient.ExecCreateOptions{
		User:         *p.Config.RemoteUser,
		TTY:          false,
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          args,
	}
	if len(p.Config.RemoteEnv) > 0 {
		for name, val := range p.Config.RemoteEnv {
			execCreateOpts.Env = append(execCreateOpts.Env, fmt.Sprintf("%s=%s", name, *val))
		}
	}
	slog.Debug("creating execution context", "container", containerID, "opts", execCreateOpts)
	execCreateRes, err := c.mobyClient.ExecCreate(ctx, containerID, execCreateOpts)
	if err != nil {
		slog.Error("encountered error while preparing execution context", "error", err)
		return err
	}
	slog.Debug("executing command", "container", containerID, "context", execCreateRes.ID)
	execAttachRes, err := c.mobyClient.ExecAttach(ctx, execCreateRes.ID, mobyclient.ExecAttachOptions{})
	if err != nil {
		slog.Error("encountered error while executing the command", "error", err)
		return err
	}
	execInspectRes, err := c.mobyClient.ExecInspect(ctx, execCreateRes.ID, mobyclient.ExecInspectOptions{})
	if err != nil {
		slog.Error("encountered error while inspecting execution context", "error", err)
		return err
	}

	cmdStderr := bytes.Buffer{}
	cmdStdout := bytes.Buffer{}
	_, err = stdcopy.StdCopy(&cmdStdout, &cmdStderr, execAttachRes.Reader)
	if err != nil {
		slog.Error("could not demultiplex output from command", "cmd", cmd, "error", err)
		return err
	}

	slog.Info("command output", "cmd", cmd, "stdout", cmdStdout.String(), "stderr", cmdStderr.String())
	if execInspectRes.ExitCode != 0 {
		slog.Error("command ran in container returned non-zero", "exit-code", execInspectRes.ExitCode, "cmd", cmd)
		err = fmt.Errorf("command returned non-zero exit code: %d", execInspectRes.ExitCode)
	}

	return err
}

// StartDevcontainerContainer starts and attaches to a container based
// on configuration from devcontainer.json.
//
// Requires metadata parsed from a devcontainer.json config, the
// tag/image name for the OCI image to use as base, and a name for the
// created container.
func (c *Client) StartDevcontainerContainer(p *writ.DevcontainerParser, imageTag string, containerName string) error {
	slog.Debug("attempting to start and attach to devcontainer", "tag", imageTag, "name", containerName)
	containerCfg := c.buildContainerConfig(p, imageTag)
	hostCfg := c.buildHostConfig(p)

	if err := c.bindAppPorts(p, containerCfg, hostCfg); err != nil {
		slog.Error("encountered an error binding appPorts items", "error", err)
		return err
	}

	return c.StartContainer(p, containerCfg, hostCfg, containerName, true)
}

// StartContainer creates a container based on the passed in arguments
// then starts it.
func (c *Client) StartContainer(p *writ.DevcontainerParser, containerCfg *container.Config, hostCfg *container.HostConfig, containerName string, isDevcontainer bool) error {
	if isDevcontainer {
		p.DevcontainerID = &c.ContainerID

		if err := c.bindForwardPorts(p, containerCfg, hostCfg); err != nil {
			slog.Error("encountered an error binding forwardPorts items", "error", err)
			return err
		}
		c.bindMounts(p, hostCfg)

		if err := c.setContainerAndRemoteUser(p, containerCfg.Image); err != nil {
			slog.Error("encountered an error while attempting to determine container/remote user", "image", containerCfg.Image, "error", err)
			return err
		}

		if *p.Config.UpdateRemoteUserUID {
			if *p.Config.ContainerUser == "root" {
				// This doesn't seem to faze Docker (tested on Windows 11
				// + Docker Desktop 4.55.0 (213807)) like I thought it
				// would, so I'm just gonna leave this is.
				hostCfg.UsernsMode = "keep-id:uid=0,gid=0"
			}
			// TODO: Add logic for the non-root scenario
		}

		// Lifecycle: featureInstall
		c.DevcontainerLifecycleChan <- LifecycleFeatureInstall
		if ok := <-c.DevcontainerLifecycleResp; !ok {
			return ErrLifecycleHandler
		}

		// Lifecycle: initialize
		c.DevcontainerLifecycleChan <- LifecycleInitialize
		if ok := <-c.DevcontainerLifecycleResp; !ok {
			return ErrLifecycleHandler
		}
	}

	slog.Debug("using container config", "config", containerCfg)
	slog.Debug("using host config", "config", hostCfg)

	ctx := context.Background()
	createResp, err := c.mobyClient.ContainerCreate(ctx, mobyclient.ContainerCreateOptions{
		Config:     containerCfg,
		HostConfig: hostCfg,
		Name:       containerName,
		Platform:   (*ocispec.Platform)(&c.Platform),
	})
	if err != nil {
		slog.Error("encountered an error creating a container", "error", err)
		return err
	}
	slog.Debug("container created successfully", "id", createResp.ID)

	if isDevcontainer {
		c.ContainerID = createResp.ID

		// "Cheat" a little bit by attaching to the container immediately
		// after creation.
		//
		// Attaching to a container before it even starts prevents missing
		// a log replay upon attachment.
		//
		// A symptom of that is needing to input something
		// after the container is attached to, to get, say, the shell
		// prompt to appear.
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
		c.attachResp = &attachResp
	}

	slog.Debug("attempting to start container", "id", createResp.ID)
	// TODO: Support the container initialization options/operations
	// exposed by the devcontainer spec
	if _, err := c.mobyClient.ContainerStart(ctx, createResp.ID, mobyclient.ContainerStartOptions{}); err != nil {
		slog.Error("encountered an error while trying to start the container", "error", err)
		return err
	}
	slog.Debug("container started successfully", "id", createResp.ID)

	if isDevcontainer {
		// Lifecycle hooks
		c.DevcontainerLifecycleChan <- LifecycleOnCreate
		if ok := <-c.DevcontainerLifecycleResp; !ok {
			return ErrLifecycleHandler
		}
		c.DevcontainerLifecycleChan <- LifecycleUpdate
		if ok := <-c.DevcontainerLifecycleResp; !ok {
			return ErrLifecycleHandler
		}
		c.DevcontainerLifecycleChan <- LifecyclePostCreate
		if ok := <-c.DevcontainerLifecycleResp; !ok {
			return ErrLifecycleHandler
		}
		c.DevcontainerLifecycleChan <- LifecyclePostStart
		if ok := <-c.DevcontainerLifecycleResp; !ok {
			return ErrLifecycleHandler
		}
	}

	return nil
}

// StopDevcontainer signals the devcontainer to terminate and then
// subsequently removed.
//
// There is normally no reason to call this directly: this is intended
// to assist with cleanup when errors are encountered.
func (c *Client) StopDevcontainer() {
	if _, err := c.mobyClient.ContainerStop(context.Background(), c.ContainerID, mobyclient.ContainerStopOptions{}); err != nil {
		slog.Error("encountered an error while trying to stop the devcontainer", "error", err)
	}
}

// AttachHostTerminalToDevcontainer attempts to route input from the
// terminal into the container's pseudo-TTY, and redirect the
// pseudo-TTY's output to the host terminal.
//
// This allows usage of the container in a terminal as one would,
// e.g., a regular shell
func (c *Client) AttachHostTerminalToDevcontainer() (err error) {
	defer func() {
		close(c.DevcontainerLifecycleChan)
	}()

	slog.Debug("attempting to attach host terminal to container", "container", c.ContainerID)
	if c.attachResp == nil {
		return fmt.Errorf("attempted to attach host terminal without a container connection")
	}

	if c.isAttached {
		slog.Debug("attempt to attach host terminal when it's already attached; no-op")
		return nil
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("stdin is not a terminal")
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("stdout is not a terminal")
	}

	c.isAttached = true

	slog.Debug("attempting to resize container's pseudo-TTY")
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		slog.Error("encountered an error trying to get the terminal's dimensions", "error", err)
		return err
	}

	if err = c.ResizeContainer(uint(h), uint(w)); err != nil { // #nosec G115
		return err
	}
	slog.Debug("setting up hooks to handle terminal resizing")
	c.listenForTerminalResize()

	slog.Debug("setting host terminal to raw mode")
	restoreTerm, err := c.switchTerminalToRaw()
	if err != nil {
		return err
	}
	defer restoreTerm()

	slog.Debug("setting up terminal input/output")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(os.Stdout, c.attachResp.Reader); err != nil && err != io.EOF {
			slog.Error("encountered an error copying container output to stdout", "error", err)
		}
	}()
	go func() {
		if _, err := io.Copy(c.attachResp.Conn, os.Stdin); err != nil && !errors.Is(err, syscall.EPIPE) {
			slog.Error("encountered an error copying terminal input to container", "error", err)
		}
	}()

	c.DevcontainerLifecycleChan <- LifecyclePostAttach
	if ok := <-c.DevcontainerLifecycleResp; !ok {
		return ErrLifecycleHandler
	}

	wg.Wait()
	slog.Debug("detached from container", "id", c.ContainerID)

	return nil
}

// ResizeContainer sets the container's internal pseudo-TTY height and
// width to the passed in values.
func (c *Client) ResizeContainer(h uint, w uint) (err error) {
	_, err = c.mobyClient.ContainerResize(context.Background(), c.ContainerID, mobyclient.ContainerResizeOptions{
		Height: h,
		Width:  w,
	})
	return err
}

// buildContainerConfig initializes and returns a Moby
// container.Config struct for later use with containers.
func (c *Client) buildContainerConfig(p *writ.DevcontainerParser, tag string) *container.Config {
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
func (c *Client) buildHostConfig(p *writ.DevcontainerParser) *container.HostConfig {
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
func (c *Client) bindAppPorts(p *writ.DevcontainerParser, containerCfg *container.Config, hostCfg *container.HostConfig) error {
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
func (c *Client) bindForwardPorts(p *writ.DevcontainerParser, containerCfg *container.Config, hostCfg *container.HostConfig) error {
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
func (c *Client) bindMounts(p *writ.DevcontainerParser, hostCfg *container.HostConfig) {
	for _, mountEntry := range p.Config.Mounts {
		hostCfg.Mounts = append(hostCfg.Mounts, (mount.Mount)(*mountEntry))
	}
}

// setContainerAndRemoteUser tries to determine what value the
// containerUser and remoteUser fields should have based on a target
// image, provided they're not already set.
func (c *Client) setContainerAndRemoteUser(p *writ.DevcontainerParser, imageTag string) (err error) {
	if p.Config.ContainerUser == nil {
		slog.Info("containerUser not set; attempting to figure it out using image metadata")
		var imageCfg *imagespec.DockerOCIImageConfig
		if imageCfg, err = c.InspectImage(imageTag); err == nil {
			imageUser := imageCfg.User
			if len(imageUser) == 0 {
				imageUser = "root"
			}
			p.Config.ContainerUser = &imageUser
		}
	} else {
		slog.Debug("containerUser already set; skipping image metadata inspection", "user", *p.Config.ContainerUser)
	}

	if err == nil && p.Config.RemoteUser == nil {
		slog.Info("remoteUser not set; setting to be the same as containerUser", "user", *p.Config.ContainerUser)
		p.Config.RemoteUser = p.Config.ContainerUser
	}

	return err
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
