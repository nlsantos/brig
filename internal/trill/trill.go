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

	composetypes "github.com/compose-spec/compose-go/types"
	"github.com/heimdalr/dag"
	mobyclient "github.com/moby/moby/client"
)

// PrivilegedPortElevator is a function that Client can use to convert
// privileged ports it encounters into non-privileged ports.
//
// It is passed the privileged port number and the return value is
// used in the original port's stead.
//
// There is no check performed on the return value to see if it
// actually produces a port number beyond the privileged port range.
type PrivilegedPortElevator func(uint16) uint16

// Client holds metadata for communicating with Podman/Docker.
type Client struct {
	ContainerID            string                 // The internal ID the API assigned to the created container
	MakeMeRoot             bool                   // If true, will ensure that the current user gets mapped as root inside the container
	Platform               Platform               // Platform details for any containers created
	PrivilegedPortElevator PrivilegedPortElevator // If non-nil, will be called whenever a binding for a port number < 1024 is encountered; its return value will be used in place of the original port
	SocketAddr             string                 // The socket/named pipe used to communicate with the server

	mobyClient      *mobyclient.Client
	composerProject *composetypes.Project
	servicesDAG     *dag.DAG
}

// Platform contains data on the target state of any created
// containers
type Platform struct {
	Architecture string
	OS           string
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
		c.mobyClient = mobyClient
		defer func() {
			if err := c.mobyClient.Close(); err != nil {
				slog.Error("could not close Moby client", "error", err)
			}
		}()
	} else {
		panic(err)
	}

	return c
}
