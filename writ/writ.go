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

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/tailscale/hujson"
)

// DefWorkspacePath is the default path to which the context directory
// will be mounted inside the container.
//
// This deviates a little bit from the exhibited behavior of Visual
// Studio Code; under VSCode, this value changes depdending on factors
// I'm not entirely clear on.
//
// It seems to change depending on whether your code utilizes VSCode's
// [workspaces](https://code.visualstudio.com/docs/editing/workspaces/workspaces)
// feature and possibly other things.
//
// As this is not an applicable concept to brig, I've chosen to pin it
// to a known value instead.
const DefWorkspacePath string = "/workspace"

// A Parser contains information about a JSON configuration necessary
// to validate it against its corresponding JSON Schema spec.
type Parser struct {
	Filepath      string // Path to the target JSON file
	IsValidConfig bool   // Whether or not the contents of the JSON file conforms to its corresponding spec

	defaultValues    map[string]any // Default values of various fields keyed by their name
	jsonSchema       string         // Contents of the JSON schema to validate against
	jsonSchemaPath   string         // Path used for the JSON schema when being added as a resource
	standardizedJSON []byte         // The raw contents of the target devcontainer.json, converted to standard JSON
}

// A DevcontainerParser contains metadata about a target
// devcontainer.json file, as well as the configuration for the
// intended devcontainer itself.
type DevcontainerParser struct {
	Config         DevcontainerConfig // The parsed contents of the target devcontainer.json
	DevcontainerID *string            // The runtime-specific ID for the devcontainer; not available until after it's created

	Parser
}

func NewParser(configPath string) (p *Parser, err error) {
	if configPath, err = filepath.Abs(configPath); err != nil {
		return nil, err
	}
	p = &Parser{
		Filepath:      configPath,
		IsValidConfig: false,
		defaultValues: make(map[string]any),
	}
	if err = p.standardizeJSON(); err != nil {
		return nil, err
	}
	return p, nil
}

// Validate runs the contents of the target devcontainer.json against
// a snapshot of the devcontainer spec's official JSON Schema.
//
// A successful validation operation returns err == nil and sets
// p.IsValidConfig accordingly. Until after this is run, the value of
// p.IsValidConfig should not be considered definitive.
func (p *Parser) Validate() error {
	slog.Debug("initializing JSON schema validator")
	dcSchema, err := jsonschema.UnmarshalJSON(strings.NewReader(p.jsonSchema))
	if err != nil {
		slog.Error("unable to unmarshal embedded JSON schema", "error", err)
		return err
	}
	c := jsonschema.NewCompiler()
	if err = c.AddResource(p.jsonSchemaPath, dcSchema); err != nil {
		slog.Error("unable to add embedded JSON schema as resource", "error", err)
		return err
	}
	sch, err := c.Compile(p.jsonSchemaPath)
	if err != nil {
		slog.Error(fmt.Sprintf("unable to compile JSON schema: %#v", err))
		return err
	}

	slog.Debug("unmarshalling devcontainer.json", "path", p.Filepath)
	valInput, err := jsonschema.UnmarshalJSON(bytes.NewReader(p.standardizedJSON))
	if err != nil {
		slog.Error("failed to unmarshal JSON for validation", "error", err)
		return err
	}

	if err = sch.Validate(valInput); err != nil {
		slog.Error("specified devcontainer.json failed schema validation", "path", p.Filepath)
		return err
	}

	p.IsValidConfig = true
	return nil
}

// Convert the contents of the target JSON config, which could be
// JSONC, into standard JSON suitable for validation and parsing.
func (p *Parser) standardizeJSON() error {
	slog.Debug("attempting to standardize JSON config contents", "path", p.Filepath)
	file, err := os.Open(p.Filepath)
	if err != nil {
		slog.Error("failed to open JSON config", "error", err, "path", p.Filepath)
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("could not close JSON file while standardizing", "error", err)
		}
	}()

	fileInput, err := io.ReadAll(file)
	if err != nil {
		slog.Error("failed to read contents of JSON config", "error", err, "path", p.Filepath)
		return err
	}

	if p.standardizedJSON, err = hujson.Standardize(fileInput); err != nil {
		slog.Error("failed to standardize JSON config contents", "error", err, "path", p.Filepath)
		return err
	}

	return nil
}
