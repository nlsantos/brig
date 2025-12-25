//go:build windows

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
	"log/slog"
	"os"
	"time"

	"golang.org/x/term"
)

// Hooks into terminal resize signals on Windows
func (c *Client) listenForTerminalResize() {
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		fd := int(os.Stdout.Fd())
		if !term.IsTerminal(fd) {
			slog.Debug("not a terminal", "fd", fd)
			return
		}

		for range ticker.C {
			w, h, err := term.GetSize(fd)
			if err != nil {
				slog.Error("could not get terminal's size", "error", err)
				return
			}
			c.ResizeContainer(uint(h), uint(w)) // #nosec G115
		}
	}()
}
