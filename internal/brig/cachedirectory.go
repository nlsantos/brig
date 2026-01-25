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

// Package brig houses a CLI tool for working with devcontainer.json
package brig

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"mvdan.cc/sh/v3/shell"
)

// getCacheDirectoryBase checks each entry in `prefixes` directories
// for the existence of a subdirectory named `cmd.appName`; if found,
// immediately returns it.
//
// If none of the entries in `prefixes` contain the desired
// subdirectory, it then tries to create it in the first non-empty and
// valid (i.e., exists) prefix.
//
// If none of the prefixes are actually valid paths, attempts to
// create the directory hierarchy nominated in the `fallbackPattern`
// parameter, which should be a `fmt` string to which `cmd.appName` is
// applied.
func (cmd *Command) getCacheDirectoryBase(prefixes []string, fallbackPattern string) (string, error) {
	for _, prefix := range prefixes {
		slog.Debug("attempting to resolve raw prefix", "prefix", prefix)
		cacheDirPrefix, err := shell.Expand(prefix, nil)
		if err != nil {
			slog.Error("encountered an error while attempting to resolve raw prefix", "error", err)
			return "", err
		}

		if len(cacheDirPrefix) == 0 {
			continue
		}

		if _, err := os.Stat(cacheDirPrefix); errors.Is(err, fs.ErrNotExist) {
			slog.Debug("cache prefix does not exist", "prefix", cacheDirPrefix)
			continue
		}

		cacheDir, err := filepath.Abs(filepath.Join(cacheDirPrefix, cmd.appName))
		if err != nil {
			slog.Error("encountered an error while attempting to resolve app cache path using prefix", "prefix", cacheDirPrefix, "error", err)
			return "", err
		}

		if _, err := os.Stat(cacheDir); errors.Is(err, fs.ErrNotExist) {
			slog.Debug("prefix exists, but not the app-specific subdirectory; attempting to create", "path", cacheDir)
			// Prefix exists but not the app-specific subdirectory
			if err := os.Mkdir(cacheDir, fs.ModeDir); err != nil {
				slog.Error("encountered an error while attempting to create app cache directory", "path", cacheDir, "error", err)
				return "", err
			}
		} else {
			slog.Debug("app-specific cache directory already exists", "path", cacheDir)
		}
		return cacheDir, nil
	}

	// None of the prefixes exist, so just create the fallback path
	// (including its complete hierarchy).
	fallbackCachePath, err := shell.Expand(fmt.Sprintf(fallbackPattern, cmd.appName), nil)
	if err != nil {
		slog.Error("encountered an error while attempting to resolve harcoded cache path", "error", err)
		return "", err
	}
	slog.Debug("cache prefixes list exhausted; resorting to hardcoded path", "path", fallbackCachePath)

	// This *shouldn't* ever happen (because we already check
	// for its existence above), but... *shrug*
	if _, err := os.Stat(fallbackCachePath); err == nil {
		return fallbackCachePath, nil
	}

	if err := os.MkdirAll(fallbackCachePath, fs.ModeDir); err == nil {
		return fallbackCachePath, nil
	}

	return "", err
}
