//go:build !windows

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

func (cmd *Command) getCacheDirectory() (string, error) {
	prefixes := []string{
		"${XDG_DATA_HOME}",
		"${XDG_CACHE_HOME}",
		// Maybe XDG env vars just aren't declared?
		"${HOME}/.local/share",
		"${HOME}/.cache",
	}
	fallbackPattern := "${HOME}/.local/share/%s"
	return cmd.getCacheDirectoryBase(prefixes, fallbackPattern)
}
