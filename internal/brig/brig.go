/*
   brig: The lightweight, native Go CLI for devcontainers
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

// Package brig houses a CLI tool for wokring with devcontainer.json
package brig

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/MakeNowJust/heredoc"
	"github.com/go-git/go-git/v6"
	"github.com/golang-cz/devslog"
	imagespec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/nlsantos/brig/internal/trill"
	"github.com/nlsantos/brig/writ"
	"github.com/pborman/options"
	"golang.org/x/sync/errgroup"
)

// ExitCode is a list of numeric exit codes used by brig
type ExitCode uint

// Exiting brig returns one of these values to the shell
const (
	ExitNormal ExitCode = iota
	ExitError
	ExitNonValidDevcontainerJSON
	ExitNoSocketFound
	ExitErrorParsingFlags
	ExitNoDevcJSONFound
	ExitTooManyDevJSONFound
	ExitUnsupportedConfiguration
)

// ImageTagPrefix is the default prefix used for the tag of images
// built by brig
const ImageTagPrefix = "localhost/devc--"

// PrivilegedPortOffset is added to privileged port bindings when they
// are encountered, in order to raise them past 1023
//
// e.g., if attempting to bind port 53 on the host, it will be
// translated as (53 + PortElevationFactor) before binding.
const PrivilegedPortOffset uint16 = 8000

// StandardDevcontainerJSONPatterns is a list of paths and globs where
// devcontainer.json files could reside.
//
// Based on
// https://containers.dev/implementors/spec/#devcontainerjson; update
// as necessary.
var StandardDevcontainerJSONPatterns = []string{
	".devcontainer.json",
	".devcontainer/devcontainer.json",
	".devcontainer/*/devcontainer.json",
}

// VersionText is just the message printed out when verison
// information is requested.
var VersionText = heredoc.Doc(`
    %s, version %s
    The lightweight, native Go CLI for devcontainers
    Copyright (C) 2025  Neil Santos

    License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>

    This is free software; you are free to change and redistribute it.
    There is NO WARRANTY, to the extent permitted by law.
`)

// Command holds state useful in brig's operations
type Command struct {
	Arguments []string
	Options   struct {
		Help         options.Help  `getopt:"-h --help display this help message"`
		Config       options.Flags `getopt:"-c --config=PATH path to rc file"`
		Debug        bool          `getopt:"-d --debug enable debug messsages (implies -v)"`
		MakeMeRoot   bool          `getopt:"-R --make-me-root map your UID to root in the container (Podman-only)"`
		PlatformArch string        `getopt:"-a --platform-arch target architecture for the container; defaults to amd64"`
		PlatformOS   string        `getopt:"-o --platform-os target operating system for the container; defaults to linux"`
		PortOffset   uint16        `getopt:"-p --port-offset=UINT number to offset privileged ports by"`
		Socket       string        `getopt:"-s --socket=ADDR URI to the Podman/Docker socket"`
		ValidateOnly bool          `getopt:"-V --validate parse and validate  the config and exit immediately"`
		Verbose      bool          `getopt:"-v --verbose enable diagnostic messages"`
		Version      bool          `getopt:"--version display version informaiton then exit"`
	}

	suppressOutput bool
}

// NewCommand initializes the command's lifecycle
func NewCommand(appName string, appVersion string) ExitCode {
	var cmd Command
	var err error

	cmd.parseOptions(appName, appVersion)
	slog.Debug("command line options parsed", "opts", cmd.Options)
	slog.Debug("command line arguments ", "args", cmd.Arguments)

	targetDevcontainerJSON := findDevcontainerJSON(cmd.Arguments)
	slog.Debug("instantiating a parser for devcontainer.json", "path", targetDevcontainerJSON)

	parser := writ.NewParser(targetDevcontainerJSON)
	if err = parser.Validate(); err != nil {
		slog.Error("devcontainer.json has syntax errors", "path", targetDevcontainerJSON, "error", err)
		return ExitNonValidDevcontainerJSON
	}
	if err = parser.Parse(); err != nil {
		slog.Error("devcontainer.json could not be parsed", "path", targetDevcontainerJSON, "error", err)
		return ExitNonValidDevcontainerJSON
	}
	if cmd.Options.ValidateOnly {
		slog.Info("devcontainer.json validated and parsed successfully", "path", targetDevcontainerJSON)
		return ExitNormal
	}

	socketAdddr := getSocketAddr(cmd.Options.Socket)
	if len(socketAdddr) == 0 {
		slog.Error("No socket address / path specified and none can be found")
		fmt.Println("fatal: Could not determine Podman/Docker socket address. Exiting.")
		return ExitNoSocketFound
	}

	trillClient := trill.NewClient(socketAdddr, cmd.Options.MakeMeRoot)
	trillClient.Platform = trill.Platform{
		Architecture: cmd.Options.PlatformArch,
		OS:           cmd.Options.PlatformOS,
	}
	trillClient.PrivilegedPortElevator = cmd.privilegedPortElevator
	defer func() {
		if parser.Config.DockerComposeFile == nil {
			if len(trillClient.ContainerID) > 0 {
				trillClient.StopDevcontainer()
			}
		} else if err = trillClient.TeardownComposerProject(); err != nil {
			slog.Error("encountered an error while trying to tear down the Compose project", "error", err)
		}

		if err = trillClient.Close(); err != nil {
			slog.Error("received an error while closing the trill client", "error", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		defer cancel()
		return cmd.lifecycleHandler(eg, egCtx, trillClient, &parser)
	})
	eg.Go(func() (err error) {
		imageName := createImageTagBase(&parser)
		var imageTag string
		switch {
		case parser.Config.DockerFile != nil && len(*parser.Config.DockerFile) > 0:
			imageTag = fmt.Sprintf("%s%s", ImageTagPrefix, imageName)
			if err = trillClient.BuildDevcontainerImage(&parser, imageTag, cmd.suppressOutput); err != nil {
				slog.Error("encountered an error while trying to build an image based on devcontainer.json", "error", err)
				return err
			}
			if err = cmd.setContainerAndRemoteUser(&parser, trillClient, imageTag); err != nil {
				slog.Error("encountered an error while attempting to determine container/remote user", "image", imageTag, "error", err)
				return err
			}
			if err = trillClient.StartDevcontainerContainer(&parser, imageTag, imageName); err != nil {
				slog.Error("encountered an error while trying to start the devcontainer", "error", err)
				return err
			}

		case parser.Config.DockerComposeFile != nil && len(*parser.Config.DockerComposeFile) > 0:
			slog.Warn("SUPPORT FOR COMPOSER PROJECTS IS INCOMPLETE")
			if err = trillClient.DeployComposerProject(&parser, imageName, ImageTagPrefix, cmd.suppressOutput); err != nil {
				slog.Error("encountered an error while trying to build a Compose project", "error", err)
			}

		case parser.Config.Image != nil && len(*parser.Config.Image) > 0:
			imageTag = *parser.Config.Image
			if err = trillClient.PullContainerImage(imageTag, cmd.suppressOutput); err != nil {
				slog.Error("encountered an error while trying to pull an image based on devcontainer.json", "error", err)
				return err
			}
			if err = cmd.setContainerAndRemoteUser(&parser, trillClient, imageTag); err != nil {
				slog.Error("encountered an error while attempting to determine container/remote user", "image", imageTag, "error", err)
				return err
			}
			if err = trillClient.StartDevcontainerContainer(&parser, imageTag, imageName); err != nil {
				slog.Error("encountered an error while trying to start the devcontainer", "error", err)
			}

		default:
			return fmt.Errorf("devcontainer.json specifies an unsupported mode of operation; exiting")
		}
		return err
	})

	if err = eg.Wait(); err != nil {
		slog.Error("errgroup encountered an error", "error", err)
		return ExitError
	}

	slog.Debug("exiting cleanly")
	return ExitNormal
}

// Try to generate a distinct yet meaningful name for the generated
// OCI image based on available metadata.
//
// If the context directory is a git repository, this function will
// build a name using various git-related information; otherwise, it
// defaults to the basename of the contect directory.
func createImageTagBase(p *writ.Parser) string {
	// Use the basename of the devcontainer.json's context as default
	// value
	ctxDir := *p.Config.Context
	retval := filepath.Base(ctxDir)

	// Attempt to open the repository in the current directory
	openOpts := git.PlainOpenOptions{
		DetectDotGit:          true,
		EnableDotGitCommonDir: true,
	}
	repo, err := git.PlainOpenWithOptions(ctxDir, &openOpts)
	if err != nil {
		slog.Debug("does not seem to be in a git repo; using default")
		return retval
	}

	cfg, err := repo.Config()
	if err != nil {
		slog.Error(fmt.Sprintf("could not open git repo configuration: %v", err))
		return retval
	}

	// Try to get the URL of the origin remote
	remote, ok := cfg.Remotes["origin"]
	if !ok {
		slog.Error("remote named 'origin' not found")
		return retval
	}

	repoURL := remote.URLs[0]
	repoName := strings.TrimSuffix(filepath.Base(repoURL), ".git")

	headRef, err := repo.Head()
	if err != nil {
		slog.Error(fmt.Sprintf("unable to determine abbreviated reference name: %v", err))
		return repoName
	}

	refName := headRef.Name()
	if refName == "HEAD" {
		retval = fmt.Sprintf("%s--%s", repoName, headRef.Hash().String())
	} else {
		retval = fmt.Sprintf("%s--%s", repoName, refName.Short())
	}
	invalidContainerNamePattern := regexp.MustCompile("[^a-zA-Z0-9_.-]")
	// Replace non-valid characters for container names with an
	// underscore
	retval = invalidContainerNamePattern.ReplaceAllString(retval, "_")

	return retval
}

// findDevcontainerJSON attempts to find a suitable devcontainer.json
// given a list of path patterns and/or plain paths.
//
// paths may contain strings incorporating patterns supported by
// [filepath.Glob]
//
// If paths is empty, it attempts to find one or more valid file paths
// using StandardDevcontainerJSONPatterns. Otherwise, paths is
// iterated upon.
//
// Returns a string if a valid devcontainers.json is found; any errors
// encountered, it runs os.Exit() with the appropriate ExitCode value.
func findDevcontainerJSON(paths []string) string {
	if len(paths) == 0 {
		slog.Debug("iterating through standard devcontainer.json paths/patterns", "paths", StandardDevcontainerJSONPatterns)
		return findDevcontainerJSON(StandardDevcontainerJSONPatterns)
	}

	slog.Debug("iterating through given paths/patterns looking for a devcontainer.json", "paths", paths)
	var candidates []string
	for _, path := range paths {
		matches, err := filepath.Glob(path)
		if err != nil {
			panic(err)
		}

		for _, match := range matches {
			if _, err := os.Stat(match); err != nil {
				continue
			}
			if abspath, err := filepath.Abs(path); err == nil {
				candidates = append(candidates, abspath)
			}
		}
	}

	switch {
	case len(candidates) == 0:
		slog.Debug("unable to find any devcontainer.json candidates")
		fmt.Println("Unable to find a valid devcontainer.json file to target; exiting.")
		os.Exit(int(ExitNoDevcJSONFound))

	case len(candidates) > 1:
		slog.Debug("found multiple devcontainer.json candidates; giving up", "candidates", candidates)
		fmt.Println(heredoc.Doc(`
			Found multiple possible devcontainer configurations.
			Specify one explicitly as an argument in the command line flag to continue.

			The following paths are eligible candidates:
		`))
		for _, target := range candidates {
			fmt.Printf("\t%s\n", target)
		}
		os.Exit(int(ExitTooManyDevJSONFound))

	default:
		slog.Debug("found a devcontainer.json to target", "path", candidates[0])
	}

	return candidates[0]
}

// lifecycleHandler monitor's the trill client's lifecycle channel and
// runs the appropriate hooks.
func (cmd *Command) lifecycleHandler(eg *errgroup.Group, ctx context.Context, c *trill.Client, p *writ.Parser) (err error) {
	defer func() {
		close(c.DevcontainerLifecycleResp)
	}()

	for event := range c.DevcontainerLifecycleChan {
		switch event {
		case trill.LifecycleInitialize:
			slog.Debug("lifecycle", "event", "init")
			if p.Config.InitializeCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.InitializeCommand, true); err != nil {
					return err
				}
			}
			if *p.Config.WaitFor == writ.WaitForInitializeCommand {
				eg.Go(c.AttachHostTerminalToDevcontainer)
			}

		case trill.LifecycleOnCreate:
			slog.Debug("lifecycle", "event", "onCreate")
			if p.Config.OnCreateCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.OnCreateCommand, false); err != nil {
					return err
				}
			}
			if *p.Config.WaitFor == writ.WaitForOnCreateCommand {
				eg.Go(c.AttachHostTerminalToDevcontainer)
			}

		case trill.LifecyclePostAttach:
			slog.Debug("lifecycle", "event", "postAttach")
			if p.Config.PostAttachCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.PostAttachCommand, false); err != nil {
					return err
				}
			}

		case trill.LifecyclePostCreate:
			slog.Debug("lifecycle", "event", "postCreate")
			if p.Config.PostCreateCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.PostCreateCommand, false); err != nil {
					return err
				}
			}
			if *p.Config.WaitFor == writ.WaitForPostCreateCommand {
				eg.Go(c.AttachHostTerminalToDevcontainer)
			}

		case trill.LifecyclePostStart:
			slog.Debug("lifecycle", "event", "postStart")
			if p.Config.PostStartCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.PostStartCommand, false); err != nil {
					return err
				}
			}
			if *p.Config.WaitFor == writ.WaitForPostStartCommand {
				eg.Go(c.AttachHostTerminalToDevcontainer)
			}

		case trill.LifecycleUpdate:
			slog.Debug("lifecycle", "event", "update")
			if p.Config.UpdateContentCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.UpdateContentCommand, false); err != nil {
					return err
				}
			}
			if *p.Config.WaitFor == writ.WaitForUpdateContentCommand {
				eg.Go(c.AttachHostTerminalToDevcontainer)
			}
		}
		c.DevcontainerLifecycleResp <- err == nil
	}

	slog.Debug("exiting lifecycle handler")
	return nil
}

// parseOptions parses the command-line options and parameters and
// does a little housekeeping.
func (c *Command) parseOptions(appName string, appVersion string) {
	options.SetDisplayWidth(80)
	options.SetHelpColumn(40)
	options.SetParameters("<path-to-devcontainer.json>")
	options.Register(&c.Options)
	c.setFlagsFile(appName)
	c.Arguments = options.Parse()

	if c.Options.Version {
		fmt.Printf(VersionText, appName, appVersion)
		os.Exit(int(ExitNormal))
	}

	logLevel := new(slog.LevelVar)
	switch {
	case c.Options.Debug:
		logLevel.Set(slog.LevelDebug)
	case c.Options.Verbose:
		logLevel.Set(slog.LevelInfo)
	default:
		logLevel.Set(slog.LevelError)
	}

	slog.SetDefault(slog.New(devslog.NewHandler(os.Stderr, &devslog.Options{
		HandlerOptions: &slog.HandlerOptions{
			AddSource: true,
			Level:     logLevel,
		},
		NewLineAfterLog:   false,
		SortKeys:          true,
		StringIndentation: true,
	})))

	if len(c.Options.PlatformArch) == 0 {
		c.Options.PlatformArch = "amd64"
	}
	slog.Info("target container architecture", "arch", c.Options.PlatformArch)

	if len(c.Options.PlatformOS) == 0 {
		c.Options.PlatformOS = "linux"
	}
	slog.Info("target container operating system", "os", c.Options.PlatformOS)

	if c.Options.PortOffset == 0 {
		c.Options.PortOffset = PrivilegedPortOffset
	} else if c.Options.PortOffset < 1024 {
		slog.Error("privileged port offset  must be >= 1024", "offset", c.Options.PortOffset)
		os.Exit(int(ExitUnsupportedConfiguration))
	}

	c.suppressOutput = logLevel.Level() > slog.LevelInfo

	if c.Options.MakeMeRoot {
		slog.Info("will be mapping your UID and GID to 0:0 inside the container")
	}
}

// privilegedPortElevator is the function called by trill when
// encountering privileged ports (ports numbered < 1024).
//
// Accepts port as input and returns a port number beyond the range of
// privileged ports.
func (c *Command) privilegedPortElevator(port uint16) uint16 {
	return port + c.Options.PortOffset
}

// runLifecycleCommand determines which parameter of a given lifecycle
// command is active and runs it.
func (c *Command) runLifecycleCommand(ctx context.Context, lc *writ.LifecycleCommand, runOnHost bool) (err error) {
	switch {
	case lc.String != nil:
		if runOnHost {
			err = c.runLifecycleCommandOnHost(ctx, true, *lc.String)
		}

	case len(lc.StringArray) > 0:
		if runOnHost {
			err = c.runLifecycleCommandOnHost(ctx, false, lc.StringArray...)
		}

	case lc.ParallelCommands != nil:
		var wg sync.WaitGroup
		errChan := make(chan error, len(*lc.ParallelCommands))
		for _, cmd := range *lc.ParallelCommands {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errChan <- c.runLifecycleCommand(ctx, &writ.LifecycleCommand{CommandBase: cmd}, runOnHost)
			}()
		}
		wg.Wait()
		close(errChan)
		for err = range errChan {
			if err != nil {
				return err
			}
		}
	}
	return err
}

// runLifecycleCommandOnHost executes a lifecycle command parameter
// locally on the host.
func (c *Command) runLifecycleCommandOnHost(ctx context.Context, runInShell bool, args ...string) error {
	var cmd *exec.Cmd

	if runInShell {
		shell := os.Getenv("SHELL")
		if len(shell) == 0 {
			shell = "/bin/sh"
		}
		slog.Info("running command via shell on host", "shell", shell, "args", args)
		args = append([]string{"-c"}, args...)
		cmd = exec.CommandContext(ctx, shell, args...)
	} else {
		slog.Info("running command directly on host", "args", args)
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	}

	out, err := cmd.CombinedOutput()
	slog.Info("command output", "cmd", cmd.String(), "output", string(out), "error", err)
	return err
}

// setFlagsFile goes through a list of supported paths for the flags
// file and assigns the first valid hit for parsing
func (c *Command) setFlagsFile(appName string) {
	var defConfigPaths = []string{
		os.ExpandEnv(fmt.Sprintf("${USERPROFILE}/.%src", appName)),
		os.ExpandEnv(fmt.Sprintf("${XDG_CONFIG_HOME}/%src", appName)),
		os.ExpandEnv(fmt.Sprintf("${HOME}/.config/%src", appName)),
		os.ExpandEnv(fmt.Sprintf("${HOME}/.%src", appName)),
	}
	for _, defConfigPath := range defConfigPaths {
		if _, err := os.Stat(defConfigPath); os.IsNotExist(err) {
			continue
		}
		if err := c.Options.Config.Set(fmt.Sprintf("?%s", defConfigPath), nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(int(ExitErrorParsingFlags))
		}
	}
}

// setContainerAndRemoteUser tries to determine what value the
// containerUser and remoteUser fields should have based on a target
// image, provided they're not already set.
func (c *Command) setContainerAndRemoteUser(p *writ.Parser, tc *trill.Client, imageTag string) (err error) {
	if p.Config.ContainerUser == nil {
		slog.Info("containerUser not set; attempting to figure it out using image metadata")
		var imageCfg imagespec.DockerOCIImageConfig
		if imageCfg, err = tc.InspectImage(imageTag); err == nil {
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
