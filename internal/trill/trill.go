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
	"fmt"
	"log/slog"
	"os"

	mobyclient "github.com/moby/moby/client"
)

// A Client holds metadata for communicating with Podman/Docker.
type Client struct {
	ContainerID string
	MobyClient  *mobyclient.Client
	SocketAddr  string
}

// NewClient returns a Client that's set to communicate with
// Podman/Docker via socketAddr.
//
// If it encounters an error creating the underlying connection, it
// panics.
func NewClient(socketAddr string) *Client {
	c := &Client{SocketAddr: getSocketAddr(socketAddr)}

	if mobyClient, err := mobyclient.New(mobyclient.WithHost(c.SocketAddr)); err == nil {
		c.MobyClient = mobyClient
		defer c.MobyClient.Close()
	} else {
		panic(err)
	}

	return c
}

// Attempt to determine a viable socket address for communicating with
// Podman/Docker.
//
// If socketAddr is non-empty, this function just returns it
// immediately. Otherwise, it attempts to look for the DOCKER_HOST
// environment variable; failing that, it builds a path that will
// usually work for a system with Podman installed.
func getSocketAddr(socketAddr string) string {
	if len(socketAddr) > 0 {
		return socketAddr
	}

	if envSocketAddr, ok := os.LookupEnv("DOCKER_HOST"); ok {
		slog.Debug("using socket nominated by DOCKER_HOST", "socket", envSocketAddr)
		return envSocketAddr
	}

	uid := os.Getuid()
	compSocketAddr := fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", uid)
	slog.Debug("falling back to computed socket address", "socket", compSocketAddr)
	return compSocketAddr
}
