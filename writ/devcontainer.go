/*
   writ: a devcontainer.json parser
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

// Package writ houses a validating parser for devcontainer.json files
package writ

// Initially generated using https://app.quicktype.io/ against
// https://raw.githubusercontent.com/devcontainers/spec/d424cc157e9a110f3bf67d311b46c7306d5a465d/schemas/devContainer.base.schema.json

import (
	"github.com/moby/moby/api/types/mount"
)

// DevcontainerConfig represents the contents of a devcontainer.json
// file.
type DevcontainerConfig struct {
	// Docker build-related options.
	Build *BuildOptions `json:"build,omitempty"`
	// The location of the context folder for building the Docker image. The path is relative to
	// the folder containing the `devcontainer.json` file.
	Context *string `json:"context,omitempty"`
	// The location of the Dockerfile that defines the contents of the container. The path is
	// relative to the folder containing the `devcontainer.json` file.
	DockerFile *string `json:"dockerFile,omitempty"`
	// The docker image that will be used to create the container.
	Image *string `json:"image,omitempty"`
	// Application ports that are exposed by the container. This can be a single port or an
	// array of ports. Each port can be a number or a string. A number is mapped to the same
	// port on the host. A string is passed to Docker unchanged and can be used to map ports
	// differently, e.g. "8000:8010".
	AppPort *AppPort `json:"appPort,omitempty"`
	// Whether to overwrite the command specified in the
	// image. Defaults to false if referencing a Composer project;
	// otherwise, defaults to true.
	OverrideCommand *bool `json:"overrideCommand,omitempty"`
	// The arguments required when starting in the container.
	RunArgs []string `json:"runArgs,omitempty"`
	// Action to take when the user disconnects from the container in their editor. The default
	// is to stop the container or Composer services.
	ShutdownAction *ShutdownAction `json:"shutdownAction,omitempty"`
	// The path of the workspace folder inside the container. This is typically the target path
	// of a volume mount in the docker-compose.yml.
	WorkspaceFolder *string `json:"workspaceFolder,omitempty"`
	// The --mount parameter for docker run. The default is to mount the project folder at
	// /workspaces/$project.
	WorkspaceMount *string `json:"workspaceMount,omitempty"`
	// The name of the docker-compose file(s) used to start the services.
	DockerComposeFile *DockerComposeFile `json:"dockerComposeFile,omitempty"`
	// An array of services that should be started and stopped.
	RunServices []string `json:"runServices,omitempty"`
	// The service you want to work on. This is considered the primary container for your dev
	// environment which your editor will connect to.
	Service *string `json:"service,omitempty"`
	// The JSON schema of the `devcontainer.json` file.
	Schema               *string                `json:"$schema,omitempty"`
	AdditionalProperties map[string]interface{} `json:"additionalProperties,omitempty"`
	// Passes docker capabilities to include when creating the dev container.
	CapAdd []string `json:"capAdd,omitempty"`
	// Container environment variables.
	ContainerEnv map[string]string `json:"containerEnv,omitempty"`
	// The user the container will be started with. The default is the user on the Docker image.
	ContainerUser *string `json:"containerUser,omitempty"`
	// Tool-specific configuration. Each tool should use a JSON object subproperty with a unique
	// name to group its customizations.
	Customizations map[string]interface{} `json:"customizations,omitempty"`
	// Features to add to the dev container.
	Features FeatureMap `json:"features,omitempty"`
	// Ports that are forwarded from the container to the local machine. Can be an integer port
	// number, or a string of the format "host:port_number".
	ForwardPorts ForwardPorts `json:"forwardPorts,omitempty"`
	// Host hardware requirements.
	HostRequirements *HostRequirements `json:"hostRequirements,omitempty"`
	// Passes the --init flag when creating the dev container.
	Init *bool `json:"init,omitempty"`
	// A command to run locally (i.e Your host machine, cloud VM) before anything else. This
	// command is run before "onCreateCommand". If this is a single string, it will be run in a
	// shell. If this is an array of strings, it will be run as a single command without shell.
	// If this is an object, each provided command will be run in parallel.
	InitializeCommand *LifecycleCommand `json:"initializeCommand,omitempty"`
	// Mount points to set up when creating the container. See Docker's documentation for the
	// --mount option for the supported syntax.
	Mounts []*MobyMount `json:"mounts,omitempty"`
	// A name for the dev container which can be displayed to the user.
	Name *string `json:"name,omitempty"`
	// A command to run when creating the container. This command is run after
	// "initializeCommand" and before "updateContentCommand". If this is a single string, it
	// will be run in a shell. If this is an array of strings, it will be run as a single
	// command without shell. If this is an object, each provided command will be run in
	// parallel.
	OnCreateCommand      *LifecycleCommand `json:"onCreateCommand,omitempty"`
	OtherPortsAttributes *PortAttributes   `json:"otherPortsAttributes,omitempty"`
	// Array consisting of the Feature id (without the semantic version) of Features in the
	// order the user wants them to be installed.
	OverrideFeatureInstallOrder []string                  `json:"overrideFeatureInstallOrder,omitempty"`
	PortsAttributes             map[string]PortAttributes `json:"portsAttributes,omitempty"`
	// A command to run when attaching to the container. This command is run after
	// "postStartCommand". If this is a single string, it will be run in a shell. If this is an
	// array of strings, it will be run as a single command without shell. If this is an object,
	// each provided command will be run in parallel.
	PostAttachCommand *LifecycleCommand `json:"postAttachCommand,omitempty"`
	// A command to run after creating the container. This command is run after
	// "updateContentCommand" and before "postStartCommand". If this is a single string, it will
	// be run in a shell. If this is an array of strings, it will be run as a single command
	// without shell. If this is an object, each provided command will be run in parallel.
	PostCreateCommand *LifecycleCommand `json:"postCreateCommand,omitempty"`
	// A command to run after starting the container. This command is run after
	// "postCreateCommand" and before "postAttachCommand". If this is a single string, it will
	// be run in a shell. If this is an array of strings, it will be run as a single command
	// without shell. If this is an object, each provided command will be run in parallel.
	PostStartCommand *LifecycleCommand `json:"postStartCommand,omitempty"`
	// Passes the --privileged flag when creating the dev container.
	Privileged *bool `json:"privileged,omitempty"`
	// Remote environment variables to set for processes spawned in the container including
	// lifecycle scripts and any remote editor/IDE server process.
	RemoteEnv map[string]*string `json:"remoteEnv,omitempty"`
	// The username to use for spawning processes in the container including lifecycle scripts
	// and any remote editor/IDE server process. The default is the same user as the container.
	RemoteUser *string `json:"remoteUser,omitempty"`
	// Recommended secrets for this dev container. Recommendations are provided as environment
	// variable keys with optional metadata.
	Secrets *Secrets `json:"secrets,omitempty"`
	// Passes docker security options to include when creating the dev container.
	SecurityOpt []string `json:"securityOpt,omitempty"`
	// A command to run when creating the container and rerun when the workspace content was
	// updated while creating the container. This command is run after "onCreateCommand" and
	// before "postCreateCommand". If this is a single string, it will be run in a shell. If
	// this is an array of strings, it will be run as a single command without shell. If this is
	// an object, each provided command will be run in parallel.
	UpdateContentCommand *LifecycleCommand `json:"updateContentCommand,omitempty"`
	// Controls whether on Linux the container's user should be updated with the local user's
	// UID and GID. On by default when opening from a local folder.
	UpdateRemoteUserUID *bool `json:"updateRemoteUserUID,omitempty"`
	// User environment probe to run. The default is "loginInteractiveShell".
	UserEnvProbe *UserEnvProbe `json:"userEnvProbe,omitempty"`
	// The user command to wait for before continuing execution in the background while the UI
	// is starting up. The default is "updateContentCommand".
	WaitFor *WaitFor `json:"waitFor,omitempty"`
}

// BuildOptions represents Docker build-related options.
type BuildOptions struct {
	// The location of the context folder for building the Docker image. The path is relative to
	// the folder containing the `devcontainer.json` file.
	Context *string `json:"context,omitempty"`
	// The location of the Dockerfile that defines the contents of the container. The path is
	// relative to the folder containing the `devcontainer.json` file.
	Dockerfile *string `json:"dockerfile,omitempty"`
	// Build arguments.
	Args map[string]string `json:"args,omitempty"`
	// The image to consider as a cache. Use an array to specify multiple images.
	CacheFrom *CacheFrom `json:"cacheFrom,omitempty"`
	// Additional arguments passed to the build command.
	Options []string `json:"options,omitempty"`
	// Target stage in a multi-stage build.
	Target *string `json:"target,omitempty"`
}

// DockerComposeFile contains wither a path or an ordered list of
// paths to Docker Compose files relative to the devcontainer.json
// file.
//
// Using an array is useful when extending your Docker Compose
// configuration. The order of the array matters since the contents of
// later files can override values set in previous ones.
type DockerComposeFile []string

type FeatureMap map[string]Feature

// Feature represents additional functionality that's bolted onto a
// devcontainer.
type Feature map[string]FeatureOptions

// FeatureOptions are possible options to be passed to a devcontainer
// feature's install.sh entrypoint.
type FeatureOptions struct {
	String *string
	Bool   *bool
}

// HostRequirements represent hardware requirements of the
// devcontainer.
type HostRequirements struct {
	// Number of required CPUs.
	Cpus *int64    `json:"cpus,omitempty"`
	GPU  *GPUUnion `json:"gpu,omitempty"`
	// Amount of required RAM in bytes. Supports units tb, gb, mb and kb.
	Memory *string `json:"memory,omitempty"`
	// Amount of required disk space in bytes. Supports units tb, gb, mb and kb.
	Storage *string `json:"storage,omitempty"`
}

// GPUUnion is a union struct to represent possible input for the GPU
// configuration.
type GPUUnion struct {
	Bool     *bool
	Enum     *GPUEnum
	GPUClass *GPUClass
}

// GPUEnum represents the possible string values for the field
type GPUEnum string

// Supported values for GPUEnum
const (
	Optional GPUEnum = "optional"
)

// GPUClass indicates whether a GPU is required.
//
// The string "optional" indicates that a GPU is optional. An object
// value can be used to configure more detailed requirements.
type GPUClass struct {
	// Number of required cores.
	Cores *int64 `json:"cores,omitempty"`
	// Amount of required RAM in bytes. Supports units tb, gb, mb and kb.
	Memory *string `json:"memory,omitempty"`
}

// PortAttributes represent configuration that should be applied to a
// port binding specified in forwardPorts.
type PortAttributes struct {
	// Automatically prompt for elevation (if needed) when this port is forwarded. Elevate is
	// required if the local port is a privileged port.
	ElevateIfNeeded *bool `json:"elevateIfNeeded,omitempty"`
	// Label that will be shown in the UI for this port.
	Label *string `json:"label,omitempty"`
	// Defines the action that occurs when the port is discovered for automatic forwarding
	OnAutoForward *OnAutoForward `json:"onAutoForward,omitempty"`
	// The protocol to use when forwarding this port.
	Protocol         *Protocol `json:"protocol,omitempty"`
	RequireLocalPort *bool     `json:"requireLocalPort,omitempty"`
}

// Secrets represent recommended secrets for this dev
// container. Recommendations are provided as environment variable
// keys with optional metadata.
type Secrets struct{}

// OnAutoForward defines the action that occurs when the port is
// discovered for automatic forwarding
type OnAutoForward string

// Supported values for OnAutoForward
const (
	OnAutoForwardIgnore      OnAutoForward = "ignore"
	OnAutoForwardNotify      OnAutoForward = "notify"
	OnAutoForwardOpenBrowser OnAutoForward = "openBrowser"
	OnAutoForwardOpenPreview OnAutoForward = "openPreview"
	OnAutoForwardSilent      OnAutoForward = "silent"
)

// Protocol specifies the protocol to use when forwarding a given port.
type Protocol string

// Supported values for Protocol
const (
	ProtocolHTTP  Protocol = "http"
	ProtocolHTTPS Protocol = "https"
	// This isn't one of the explicitly defined values for this field,
	// but the spec states that if this field is unset,
	// imeplementations are expected to behave as though it's set to
	// "tcp"
	ProtocolTCP Protocol = "tcp"
)

// ShutdownAction represents the action to take when the user
// disconnects from the container in their editor. The default is to
// stop the container.
//
// Action to take when the user disconnects from the primary container in their editor. The
// default is to stop all of the compose containers.
type ShutdownAction string

// Supported values for ShutdownAction
const (
	ShutdownActionNone          ShutdownAction = "none"
	ShutdownActionStopCompose   ShutdownAction = "stopCompose"
	ShutdownActionStopContainer ShutdownAction = "stopContainer"
)

// UserEnvProbe specifies the environment probe to run.
//
// The default is "loginInteractiveShell".
type UserEnvProbe string

// Suppported values for UserEnvProbe
const (
	UserEnvProbeInteractiveShell      UserEnvProbe = "interactiveShell"
	UserEnvProbeLoginInteractiveShell UserEnvProbe = "loginInteractiveShell"
	UserEnvProbeLoginShell            UserEnvProbe = "loginShell"
	UserEnvProbeUserEnvProbeNone      UserEnvProbe = "none"
)

// WaitFor represents the user command to wait for before continuing
// execution in the background while the UI is starting up.
//
// The default is "updateContentCommand".
type WaitFor string

// Supported values for WaitFor
const (
	WaitForInitializeCommand    WaitFor = "initializeCommand"
	WaitForOnCreateCommand      WaitFor = "onCreateCommand"
	WaitForPostCreateCommand    WaitFor = "postCreateCommand"
	WaitForPostStartCommand     WaitFor = "postStartCommand"
	WaitForUpdateContentCommand WaitFor = "updateContentCommand"
)

// AppPort is a list of ports that are exposed by the container.
//
// This can be a single port or an array of ports. Each port can be a
// number or a string. A number is mapped to the same port on the
// host. A string is passed to Docker unchanged and can be used to map
// ports differently, e.g. "8000:8010".
type AppPort []string

// CacheFrom specifies the image to consider as a cache. Use an array
// to specify multiple images.
type CacheFrom struct {
	String      *string
	StringArray []string
}

// ForwardPorts is an array of port numbers that are forwarded from
// the container to the local machine.
//
// Can be an integer port number, or a string of the format
// "host:port_number".
type ForwardPorts []string

// CommandBase represents lifecycle commands that can be set up to fire in
// response to several lifecycle events.
//
// If String is non-nil, its value will be run in a shell. If
// StringArray is not empty, its values will be run as a single
// command without shell.
type CommandBase struct {
	String      *string
	StringArray []string
}

// LifecycleCommand represents commands that can be set up to fire in response
// to several lifecycle events.
//
// If String is non-nil, its value will be run in a shell. If
// StringArray is not empty, its values will be run as a single
// command without shell. If this is an object, each provided command
// will be run in parallel.
type LifecycleCommand struct {
	CommandBase
	ParallelCommands *map[string]CommandBase
}

// MobyMount is a thin wrapper around the Moby Mount struct to allow
// writing an unmarshaller.
type MobyMount mount.Mount
