//go:build windows

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
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Attempt to determine a viable socket address for communicating with
// Podman/Docker.
//
// If socketAddr is non-empty, this function just returns it
// immediately. Otherwise, it attempts to check if certain named pipes exist; if
// one of them does, returns the string.  If no viable named pipes are found,
// returns an empty string.
func getSocketAddr(socketAddr string) string {
	if len(socketAddr) > 0 {
		slog.Debug("received a non-empty socket address", "socket", socketAddr)
		return socketAddr
	}

	const pipeProto string = "npipe://"
	retval := ""
	possibleNamedPipes := []string{
		`\\.\pipe\podman-machine-default`,
		`\\.\pipe\docker_engine`,
	}

	for _, possibleNamedPipe := range possibleNamedPipes {
		if _, err := os.Stat(possibleNamedPipe); err == nil {
			slog.Debug("using possible named pipe found in filesystem", "named-pipe", possibleNamedPipe)
			retval = possibleNamedPipe
		}
	}

	if len(retval) == 0 {
		slog.Error("unable to find a suitable named pipe to target")
	} else if !strings.HasPrefix(retval, pipeProto) {
		// The protocol seems to be mandatory for named pipes
		return fmt.Sprintf("%s%s", pipeProto, filepath.ToSlash(retval))
	}

	return retval
}
