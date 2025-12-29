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
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/go-archive"
	mobyclient "github.com/moby/moby/client"
	"github.com/moby/patternmatcher/ignorefile"
	"github.com/nlsantos/brig/writ"
	"golang.org/x/term"
)

// BuildContainerImage builds the OCI image to be used by the
// devcontainer.
//
// Requires metadata parsed from a devccontainer.json configuration
// file and a tag to apply to the built OCI image.
//
// TODO: Add a flag to toggle deletion of the context tarball after
// the creation of the OCI image
func (c *Client) BuildContainerImage(contextPath string, dockerfilePath string, imageTag string, buildOpts *mobyclient.ImageBuildOptions, suppressOutput bool) (err error) {
	slog.Debug("building container image", "tag", imageTag)
	fmt.Printf("Building image and tagging it as %s...\n", imageTag)

	// While it's possible to have the REST API build an OCI image
	// without having an intermediary tarball, I like having it around
	// so it's easier to debug issues pertaining to the context
	// tarball.
	contextArchivePath, err := buildContextArchive(contextPath)
	if err != nil {
		return err
	}
	contextArchive, err := os.Open(contextArchivePath)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			return
		}

		// contextArchive is closed automatically by the ImageBuild
		// API call
		if err = os.Remove(contextArchive.Name()); err != nil {
			slog.Error("failed cleaning up context archive", "path", contextArchive.Name(), "error", err)
			return
		}
	}()

	if buildOpts == nil {
		buildOpts = &mobyclient.ImageBuildOptions{
			Context:        contextArchive,
			Dockerfile:     dockerfilePath,
			Remove:         true,
			SuppressOutput: suppressOutput,
			Tags:           []string{imageTag},
		}
	} else {
		buildOpts.Context = contextArchive
	}
	// TODO: Support more of the build options offered by the
	// devcontainer spec
	buildResp, err := c.mobyClient.ImageBuild(context.Background(), contextArchive, *buildOpts)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			return
		}

		if err = buildResp.Body.Close(); err != nil {
			slog.Error("could not close build response", "error", err)
		}
	}()

	if suppressOutput {
		fmt.Printf("Building image using %s...\n", buildOpts.Dockerfile)
	}

	decoder := json.NewDecoder(buildResp.Body)
	for {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}

		if err = decoder.Decode(&msg); err == io.EOF {
			err = nil
			break
		} else if err != nil {
			slog.Error("error decoding JSON", "context", err)
			return err
		}

		// Maybe add fluff to the output to make it prettier?
		if msg.Stream != "" && !suppressOutput {
			PrefixedPrintf := NewPrefixedPrintff("BUILD", imageTag)
			PrefixedPrintf("%s", strings.ReplaceAll(msg.Stream, "\n", "\r\n"))
		}
		if msg.Error != "" {
			PrefixedPrintf := NewPrefixedPrintffError("BUILD")
			PrefixedPrintf("%s\r\n", msg.Error)
		}
	}

	return err
}

// BuildDevcontainerImage builds an OCI image based on options in a
// devcontainer.json.
//
// This is a very thin wrapper over BuildContainerImage.
func (c *Client) BuildDevcontainerImage(p *writ.Parser, imageTag string, suppressOutput bool) error {
	return c.BuildContainerImage(*p.Config.Context, *p.Config.DockerFile, imageTag, nil, suppressOutput)
}

// PullContainerImage pulls the OCI image from a remtoe registry so it
// can be used in the creation of a devcontainer.
//
// TODO: Implement a privilege function to support authentication so
// images can be pulled from private repositories
func (c *Client) PullContainerImage(tag string, suppressOutput bool) (err error) {
	slog.Debug("pulling image tag from remote registry", "tag", tag)
	fmt.Printf("Pulling %s from remote registry...\n", tag)
	pullResp, err := c.mobyClient.ImagePull(context.Background(), tag, mobyclient.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			return
		}

		if err := pullResp.Close(); err != nil {
			slog.Error("could not close pull response", "error", err)
		}
	}()

	if suppressOutput {
		if err := pullResp.Wait(context.Background()); err != nil {
			return err
		}
	} else {
		stdoutFd := os.Stdout.Fd()
		isTerm := term.IsTerminal(int(stdoutFd))
		streamWriter := NewPrefixedStreamWriter(os.Stdout, "PULL", tag)
		if err := jsonmessage.DisplayJSONMessagesStream(pullResp, streamWriter, stdoutFd, isTerm, nil); err != nil {
			slog.Error("error encountered while pulling image", "tag", tag, "error", err)
			return err
		}
	}

	return err
}

// buildContextExcludesList builds a list of files to be excluded in
// the creation of the context tarball.
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
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("could not close ignore file handle", "error", err)
		}
	}()

	if excludes, err = ignorefile.ReadAll(f); err != nil {
		slog.Error(fmt.Sprintf("error parsing %s; %v", ignoreFile, err))
	}
	slog.Debug(fmt.Sprintf("applying %d exclusion patterns", len(excludes)))
	return excludes
}

// buildContextArchive gathers the context directory into a tarball.
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
	defer func() {
		if err := tempFile.Close(); err != nil {
			slog.Error("could not close tempfile", "error", err)
		}
	}()

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
		Compression:      archive.Gzip,
		ExcludePatterns:  buildContextExcludesList(ctxDir),
		IncludeSourceDir: false,
		NoLchown:         true,
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
