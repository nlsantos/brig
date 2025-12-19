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
	mobyclient "github.com/moby/moby/client"
)

// A Client holds metadata for communicating with Podman/Docker.
type Client struct {
	ContainerID string
	MakeMeRoot  bool
	MobyClient  *mobyclient.Client
	SocketAddr  string
}

// NewClient returns a Client that's set to communicate with
// Podman/Docker via socketAddr.
//
// If it encounters an error creating the underlying connection, it
// panics.
func NewClient(socketAddr string, makeMeRoot bool) *Client {
	c := &Client{
		MakeMeRoot: makeMeRoot,
		SocketAddr: socketAddr,
	}

	if mobyClient, err := mobyclient.New(mobyclient.WithHost(c.SocketAddr)); err == nil {
		c.MobyClient = mobyClient
		defer c.MobyClient.Close()
	} else {
		panic(err)
	}

	return c
}
