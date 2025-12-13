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
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/go-git/go-git/v6"
	"github.com/nlsantos/brig/internal/trill"
	"github.com/nlsantos/brig/writ"
	"github.com/pborman/options"
)

// EXitCode is a list of numeric exit codes used by brig
type ExitCode int

// Exiting brig returns one of these values to the shell
const (
	ExitNormal ExitCode = iota
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
	var defConfigPath = fmt.Sprintf("${HOME}/.config/%src", appName)
	var opts = struct {
		Help    options.Help  `getopt:"-h --help display help"`
		Verbose bool          `getopt:"-v --verbose enable diagnostic messages"`
		Config  options.Flags `getopt:"-c --config=PATH path to rc file"`
		Debug   bool          `getopt:"-d --debug enable debug messsages (implies -v)"`
		Version bool          `getopt:"-V --version display version informaiton then exit"`
	}{}

	options.Register(&opts)
	if err := opts.Config.Set(fmt.Sprintf("?%s", defConfigPath), nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	args := options.Parse()

	if opts.Version {
		fmt.Printf(heredoc.Doc(`
                    %s, version %s
                    A tool for working with the devcontainer spec
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
		panic(err)
	}
	if err := parser.Parse(); err != nil {
		panic(err)
	}

	trillClient := trill.NewClient("")
	imageName := createImageTagBase(&parser)
	imageTag := fmt.Sprintf("%s%s", ImageTagPrefix, imageName)
	trillClient.BuildContainerImage(&parser, imageTag)
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
	repo, err := git.PlainOpen(ctxDir)
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
