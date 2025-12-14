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

package trill

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/moby/go-archive"
	"github.com/moby/moby/api/types/container"
	mobyclient "github.com/moby/moby/client"
	"github.com/moby/patternmatcher/ignorefile"
	"github.com/nlsantos/brig/writ"
	"golang.org/x/term"
)

// Build the OCI image to be used by the devcontainer.
//
// Requires metadata parsed from a devccontainer.json configuration
// file and a tag to apply to the built OCI image.
//
// TODO: Add a flag to toggle deletion of the context tarball after
// the creation of the OCI image
func (c *Client) BuildContainerImage(p *writ.Parser, tag string) {
	// While it's possible to have the REST API build an OCI image
	// without having an intermediary tarball, I like having it around
	// so it's easier to debug issues pertaining to the context
	// tarball.
	contextArchivePath, err := buildContextArchive(*p.Config.Context)
	if err != nil {
		panic(err)
	}
	contextArchive, err := os.Open(contextArchivePath)
	if err != nil {
		panic(err)
	}
	defer func() {
		contextArchive.Close()
		if errDefer := os.Remove(contextArchive.Name()); errDefer != nil {
			slog.Error("failed cleaning up context archive", "path", contextArchive.Name(), "error", errDefer)
		}
	}()

	// TODO: Support more of the build options offered by the
	// devcontainer spec
	buildOpts := mobyclient.ImageBuildOptions{
		Context:    contextArchive,
		Dockerfile: *p.Config.DockerFile,
		Remove:     true,
		Tags:       []string{tag},
	}
	buildResp, err := c.MobyClient.ImageBuild(context.Background(), contextArchive, buildOpts)
	if err != nil {
		panic(err)
	}
	defer buildResp.Body.Close()

	decoder := json.NewDecoder(buildResp.Body)
	for {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}

		if err := decoder.Decode(&msg); err == io.EOF {
			break
		} else if err != nil {
			slog.Error("error decoding JSON", "context", err)
			panic(err)
		}

		// Maybe add fluff to the output to make it prettier?
		if msg.Stream != "" {
			fmt.Printf("builder: %s", msg.Stream)
		}
		if msg.Error != "" {
			fmt.Printf("builder: [ERROR] %s\n", msg.Error)
		}
	}
}

// Start a container and attach to it to enable its usage.
//
// Requires metadata parsed from a devcontainer.json config, the
// tag/image name for the OCI image to use as base, and a name for the
// created container.
func (c *Client) StartContainer(p *writ.Parser, tag string, containerName string) {
	slog.Debug("attempting to start and attach to container based on tag", "tag", tag)
	containerCfg := container.Config{
		Image: tag,
		Tty:   true,
	}
	slog.Debug("using container config", "config", containerCfg)
	hostCfg := container.HostConfig{
		AutoRemove: true,
		Binds: []string{
			fmt.Sprintf("%s:%s", *p.Config.Context, *p.Config.WorkspaceFolder),
		},
	}
	slog.Debug("using host config", "config", hostCfg)
	createOpts := mobyclient.ContainerCreateOptions{
		Config:     &containerCfg,
		HostConfig: &hostCfg,
		Name:       containerName,
	}

	ctx := context.Background()
	if resp, err := c.MobyClient.ContainerCreate(ctx, createOpts); err == nil {
		c.ContainerID = resp.ID
	} else {
		panic(err)
	}
	slog.Debug("container created successfully", "id", c.ContainerID)
	// Attaching to a container before it even starts is a way to get
	// around possibly missing a log replay upon attachment. A symptom
	// of that is needing to input something after the container is
	// attached to, to get, say, the shell prompt to appear.
	slog.Debug("attempting to attach to container", "id", c.ContainerID)
	attachOpts := mobyclient.ContainerAttachOptions{
		Logs:   true,
		Stderr: true,
		Stdin:  true,
		Stdout: true,
		Stream: true,
	}
	resp, err := c.MobyClient.ContainerAttach(ctx, c.ContainerID, attachOpts)
	if err != nil {
		panic(err)
	}
	c.ResizeContainer()
	slog.Debug("successfully attached to container", "id", c.ContainerID)
	defer resp.Close()
	// Switching the terminal to raw mode ensures that input with
	// control characters (e.g., Ctrl-D) get passed through to the
	// container
	slog.Debug("switching terminal to raw mode")
	if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			panic(err)
		}
		// Hook into resize signals
		resizeCh := make(chan os.Signal, 1)
		signal.Notify(resizeCh, syscall.SIGWINCH)

		go func() {
			for range resizeCh {
				c.ResizeContainer()
			}
		}()

		defer func() {
			slog.Debug("restoring terminal state")
			signal.Stop(resizeCh)
			if err := term.Restore(fd, oldState); err != nil {
				panic(err)
			}
		}()
	}
	// This allows usage of the container in a terminal as one would,
	// e.g., a regular shell
	slog.Debug("setting up terminal input/output")
	var wg sync.WaitGroup
	wg.Go(func() {
		if _, err := io.Copy(os.Stdout, resp.Reader); err != nil {
			panic(err)
		}
	})
	go func() {
		if _, err := io.Copy(resp.Conn, os.Stdin); err != nil {
			panic(err)
		}
	}()

	slog.Debug("attempting to start container", "id", c.ContainerID)
	// TODO: Support the container initialization options/operations
	// exposed by the devcontainer spec
	if _, err := c.MobyClient.ContainerStart(ctx, c.ContainerID, mobyclient.ContainerStartOptions{}); err != nil {
		panic(err)
	}
	slog.Debug("container started successfully", "id", c.ContainerID)
	wg.Wait()
	slog.Debug("detached from container", "id", c.ContainerID)
}

// Resize the container's internal pseudo-TTY based on the current
// terminal's properties.
//
// Does nothing if stdin isn't a terminal, and panics if it encounters
// an error attempting to resize the pseudo-TTY.
func (c *Client) ResizeContainer() {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return
	}
	w, h, err := term.GetSize(fd)
	if err != nil {
		return
	}
	if _, err := c.MobyClient.ContainerResize(context.Background(), c.ContainerID, mobyclient.ContainerResizeOptions{
		// Typecasting checks turned off; if the terminal dimensions
		// ever have negative values, or values large enough to
		// overflow, I feel that that's an issue on the machine that
		// needs to be fixed, not necessarily a bug in brig
		Height: uint(h), //nolint:gosec
		Width:  uint(w), //nolint:gosec
	}); err != nil {
		panic(err)
	}
}

// Build a list of files to be excluded in the creation of the context tarball.
//
// Requires ctxDir, the path of the context directory to search
// .containerignore/.dockerignore in.
//
// This integrates support for .containerignore/.dockerignore during
// the creation of the context tarball.
//
// TODO: Investigate how Podman and Docker handle ignore files deeper
// in the context's directory structure; it might be necessary to walk
// the directory and gather all of them.
func buildContextExcludesList(ctxDir string) []string {
	slog.Debug("checking for .containerignore/.dockerignore in context directory")
	ignoreFile := filepath.Join(ctxDir, ".containerignore")
	if _, err := os.Stat(ignoreFile); os.IsNotExist(err) {
		ignoreFile = filepath.Join(ctxDir, ".dockerignore")
	}

	var excludes []string
	f, err := os.Open(ignoreFile)
	if err != nil {
		if os.IsNotExist(err) {
			return excludes
		}
		slog.Error(fmt.Sprintf("error opening %s; %v", ignoreFile, err))
		panic(err)
	}
	defer f.Close()

	if excludes, err = ignorefile.ReadAll(f); err != nil {
		slog.Error(fmt.Sprintf("error parsing %s; %v", ignoreFile, err))
	}
	slog.Debug(fmt.Sprintf("applying %d exclusion patterns", len(excludes)))
	return excludes
}

// Gather the context directory into a tarball.
//
// Creates a tarball rooted at ctxDir and returns the path to the
// created file if successful. If any errors are encountered, returns
// an empty string and the error.
//
// The created file is guaranteed to be unique in the system at the
// time of creation.
//
// While it's possible to build an OCI image without an intermediary
// file, having it makes it easier to debug issues related to the
// context tarball.
func buildContextArchive(ctxDir string) (string, error) {
	tempFile, err := os.CreateTemp("", fmt.Sprintf(".ctx-%s-*.tar.gz", filepath.Base(ctxDir)))
	slog.Debug(fmt.Sprintf("building a context archive for the container as %s", tempFile.Name()))
	if err != nil {
		panic(err)
	}
	defer tempFile.Close()

	tarOpts := &archive.TarOptions{
		// Assign ownership of files to root so we don't run into
		// namespace mapping issues when using Podman.
		//
		// TODO: Switch this over to the value of remoteUser if
		// specified in the devcontainer config.
		ChownOpts: &archive.ChownOpts{
			UID: 0,
			GID: 0,
		},
		Compression:     archive.Gzip,
		ExcludePatterns: buildContextExcludesList(ctxDir),
	}

	ctxReader, err := archive.TarWithOptions(ctxDir, tarOpts)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(tempFile, ctxReader)
	if err == nil {
		return tempFile.Name(), err
	}
	return "", err
}
