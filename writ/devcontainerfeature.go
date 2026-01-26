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
// https://raw.githubusercontent.com/devcontainers/spec/1b2baddb5f1071ca0e8bcb7eb56dbc9d3e4a674f/schemas/devContainerFeature.schema.json

// Development Container Features Metadata (devcontainer-feature.json). See
// https://containers.dev/implementors/features/ for more information.
type DevcontainerFeatureConfig struct {
	// Passes docker capabilities to include when creating the dev container.
	CapAdd []string `json:"capAdd,omitempty"`
	// Container environment variables.
	ContainerEnv map[string]string `json:"containerEnv,omitempty"`
	// Tool-specific configuration. Each tool should use a JSON object subproperty with a unique
	// name to group its customizations.
	Customizations map[string]interface{} `json:"customizations,omitempty"`
	// An object of Feature dependencies that must be satisified before this Feature is
	// installed. Elements follow the same semantics of the features object in devcontainer.json
	DependsOn map[string]interface{} `json:"dependsOn,omitempty"`
	// Indicates that the Feature is deprecated, and will not receive any further
	// updates/support. This property is intended to be used by the supporting tools for
	// highlighting Feature deprecation.
	Deprecated *bool `json:"deprecated,omitempty"`
	// Description of the Feature. For the best appearance in an implementing tool, refrain from
	// including markdown or HTML in the description.
	Description *string `json:"description,omitempty"`
	// URL to documentation for the Feature.
	DocumentationURL *string `json:"documentationURL,omitempty"`
	// Entrypoint script that should fire at container start up.
	Entrypoint *string `json:"entrypoint,omitempty"`
	// ID of the Feature. The id should be unique in the context of the repository/published
	// package where the feature exists and must match the name of the directory where the
	// devcontainer-feature.json resides.
	ID string `json:"id"`
	// Adds the tiny init process to the container (--init) when the Feature is used.
	Init *bool `json:"init,omitempty"`
	// Array of ID's of Features that should execute before this one. Allows control for feature
	// authors on soft dependencies between different Features.
	InstallsAfter []string `json:"installsAfter,omitempty"`
	// List of strings relevant to a user that would search for this definition/Feature.
	Keywords []string `json:"keywords,omitempty"`
	// Array of old IDs used to publish this Feature. The property is useful for renaming a
	// currently published Feature within a single namespace.
	LegacyIDs []string `json:"legacyIds,omitempty"`
	// URL to the license for the Feature.
	LicenseURL *string `json:"licenseURL,omitempty"`
	// Mounts a volume or bind mount into the container.
	Mounts []*MobyMount `json:"mounts,omitempty"`
	// Display name of the Feature.
	Name *string `json:"name,omitempty"`
	// A command to run when creating the container. This command is run after
	// "initializeCommand" and before "updateContentCommand". If this is a single string, it
	// will be run in a shell. If this is an array of strings, it will be run as a single
	// command without shell. If this is an object, each provided command will be run in
	// parallel.
	OnCreateCommand *LifecycleCommand `json:"onCreateCommand,omitempty"`
	// Possible user-configurable options for this Feature. The selected options will be passed
	// as environment variables when installing the Feature into the container.
	Options map[string]FeatureOption `json:"options,omitempty"`
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
	// Sets privileged mode (--privileged) for the container.
	Privileged *bool `json:"privileged,omitempty"`
	// Sets container security options to include when creating the container.
	SecurityOpt []string `json:"securityOpt,omitempty"`
	// A command to run when creating the container and rerun when the workspace content was
	// updated while creating the container. This command is run after "onCreateCommand" and
	// before "postCreateCommand". If this is a single string, it will be run in a shell. If
	// this is an array of strings, it will be run as a single command without shell. If this is
	// an object, each provided command will be run in parallel.
	UpdateContentCommand *LifecycleCommand `json:"updateContentCommand,omitempty"`
	// The version of the Feature. Follows the semanatic versioning (semver) specification.
	Version string `json:"version"`
}

// Option value is represented with a boolean value.
type FeatureOption struct {
	// Default value if the user omits this option from their configuration.
	Default *FeatureOptions `json:"default"`
	// Value as set by the parent devcontainer configuration, if any;
	// references Default unless overridden via SetOption
	Value *FeatureOptions
	// A description of the option displayed to the user by a supporting tool.
	Description *string `json:"description,omitempty"`
	// The type of the option. Can be 'boolean' or 'string'.  Options of type 'string' should
	// use the 'enum' or 'proposals' property to provide a list of allowed values.
	Type FeatureOptionType `json:"type"`
	// Allowed values for this option.  Unlike 'proposals', the user cannot provide a custom
	// value not included in the 'enum' array.
	Enum []string `json:"enum,omitempty"`
	// Suggested values for this option.  Unlike 'enum', the 'proposals' attribute indicates the
	// installation script can handle arbitrary values provided by the user.
	Proposals []string `json:"proposals,omitempty"`
}

type FeatureOptionType string

const (
	FeatureOptionTypeBoolean FeatureOptionType = "boolean"
	FeatureOptionTypeString  FeatureOptionType = "string"
)
