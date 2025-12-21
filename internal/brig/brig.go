/*
   brig: a tool for working with devcontainer.json
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
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/go-git/go-git/v6"
	"github.com/nlsantos/brig/internal/trill"
	"github.com/nlsantos/brig/writ"
	"github.com/pborman/options"
)

// ExitCode is a list of numeric exit codes used by brig
type ExitCode int

// Exiting brig returns one of these values to the shell
const (
	ExitNormal ExitCode = iota
	ExitNonValidDevcontainerJSON
	ExitNoSocketFound
	ExitErrorParsingFlags
	ExitNoDevcJSONFound
	ExitTooManyDevJSONFound
)

// A default prefix used for the tag of images built by brig
const ImageTagPrefix = "localhost/devc--"

// Based on
// https://containers.dev/implementors/spec/#devcontainerjson;
// update as necessary
var StandardDevcontainerJSONPatterns = []string{
	".devcontainer.json",
	".devcontainer/devcontainer.json",
	".devcontainer/*/devcontainer.json",
}

// NewCommand initializes the command's lifecycle
func NewCommand(appName string, appVersion string) {
	var opts = struct {
		Help         options.Help  `getopt:"-h --help display help"`
		Verbose      bool          `getopt:"-v --verbose enable diagnostic messages"`
		Config       options.Flags `getopt:"-c --config=PATH path to rc file"`
		Debug        bool          `getopt:"-d --debug enable debug messsages (implies -v)"`
		MakeMeRoot   bool          `getopt:"-R --make-me-root map your UID to root in the container (Podman-only)"`
		Socket       string        `getopt:"-s --socket=ADDR URI to the Podman/Docker socket"`
		ValidateOnly bool          `getopt:"-V --validate parse and validate  the config and exit immediately"`
		Version      bool          `getopt:"--version display version informaiton then exit"`
	}{}

	options.Register(&opts)
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
		if err := opts.Config.Set(fmt.Sprintf("?%s", defConfigPath), nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(int(ExitErrorParsingFlags))
		}
	}
	args := options.Parse()

	if opts.Version {
		fmt.Printf(heredoc.Doc(`
                    %s, version %s
                    The lightweight, native Go CLI for devcontainers
                    Copyright (C) 2025  Neil Santos

                    License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>

                    This is free software; you are free to change and redistribute it.
                    There is NO WARRANTY, to the extent permitted by law.
                `), appName, appVersion)
		os.Exit(int(ExitNormal))
	}

	logLevel := new(slog.LevelVar)
	switch {
	case opts.Debug:
		logLevel.Set(slog.LevelDebug)
	case opts.Verbose:
		logLevel.Set(slog.LevelInfo)
	default:
		logLevel.Set(slog.LevelError)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})))
	slog.Debug("command line parsed", "args", args)

	if opts.MakeMeRoot {
		slog.Info("mapping your UID and GID to 0:0 inside the container")
	}

	var targets = findDevcontainerJSON(args)
	var targetDevcontainerJSON string
	switch {
	case len(targets) == 0:
		slog.Debug("unable to find devcontainer.json candidates")
		fmt.Println("Unable to find a valid devcontainer.json file to target; exiting.")
		os.Exit(int(ExitNoDevcJSONFound))
	case len(targets) > 1:
		slog.Debug("found multiple devcontainer.json candidates; giving up")
		fmt.Println(heredoc.Doc(`
			Found multiple devcontainer specifications.
			Specify one explicitly as a value to the -f/--file command line flag to continue.

			The following paths are eligible targets:
		`))
		for _, target := range targets {
			fmt.Printf("\t%s\n", target)
		}
		os.Exit(int(ExitTooManyDevJSONFound))
	default:
		slog.Debug("found a devcontainer.json to target", "path", targets[0])
		targetDevcontainerJSON = targets[0]
	}

	slog.Debug("instantiating a parser for devcontainer.json", "path", targetDevcontainerJSON)
	parser := writ.NewParser(targetDevcontainerJSON)
	if err := parser.Validate(); err != nil {
		slog.Error("devcontainer.json has syntax errors", "path", targetDevcontainerJSON, "error", err)
		os.Exit(int(ExitNonValidDevcontainerJSON))
	}
	if err := parser.Parse(); err != nil {
		slog.Error("devcontainer.json could not be parsed", "path", targetDevcontainerJSON, "error", err)
		os.Exit(int(ExitNonValidDevcontainerJSON))
	}
	if opts.ValidateOnly {
		slog.Info("devcontainer.json validated and parsed successfully", "path", targetDevcontainerJSON)
		os.Exit(int(ExitNormal))
	}

	socketAdddr := getSocketAddr(opts.Socket)
	if len(socketAdddr) == 0 {
		slog.Error("No socket address / path specified and none can be found")
		fmt.Println("fatal: Could not determine Podman/Docker socket address. Exiting.")
		os.Exit(int(ExitNoSocketFound))
	}

	trillClient := trill.NewClient(socketAdddr, opts.MakeMeRoot)
	imageName := createImageTagBase(&parser)
	suppressOutput := logLevel.Level() > slog.LevelInfo
	var imageTag string
	if parser.Config.Image != nil && len(*parser.Config.Image) > 0 {
		imageTag = *parser.Config.Image
		slog.Debug("pulling image tag from remote registry", "tag", imageTag)
		trillClient.PullContainerImage(imageTag, suppressOutput)
	} else {
		imageTag = fmt.Sprintf("%s%s", ImageTagPrefix, imageName)
		slog.Debug("building container image", "tag", imageTag, "suppressOutput", suppressOutput)
		trillClient.BuildContainerImage(&parser, imageTag, suppressOutput)
	}

	slog.Debug("starting devcontainer", "tag", imageTag, "name", imageName)
	trillClient.StartContainer(&parser, imageTag, imageName)
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
// Returns a list of absolute paths to existing files that fit the
// above constraints.
//
// If any errors are encountered, it panics.
func findDevcontainerJSON(paths []string) []string {
	if len(paths) > 0 {
		slog.Debug("iterating through paths/patterns looking for a devcontainer.json")
		var retval []string
		for _, path := range paths {
			matches, err := filepath.Glob(path)
			if err != nil {
				panic(err)
			}
			if len(matches) < 1 {
				continue
			}
			for _, match := range matches {
				if _, err := os.Stat(match); err != nil {
					continue
				}
				if abspath, err := filepath.Abs(path); err == nil {
					retval = append(retval, abspath)
				}
			}
		}
		return retval
	}

	return findDevcontainerJSON(StandardDevcontainerJSONPatterns)
}
