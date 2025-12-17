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
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nlsantos/brig/writ/internal/writ"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/tailscale/hujson"
)

// devcontainerJSONSchema is the contents of the JSON schema against
// which devcontainer.json files are validated.
//
//go:embed specs/devContainer.base.schema.json
var devcontainerJSONSchema string

// devcontainerJSONSchemaPath is the path used for the JSON schema
// when being added manually as resource for the validator; it allows
// the schema contents to be referenced by other resources later on.
const devcontainerJSONSchemaPath = "devContainer.base.schema.json"

// A Parser contains metadata about a target devcontainer.jkson file,
// as well as the configuration for the intended devcontainer itself.
type Parser struct {
	Filepath      string                  // Path to the target devcontainer.json
	Config        writ.DevcontainerConfig // The parsed contents of the target devcontainer.json
	IsValidConfig bool                    // Whether or not the contents of the devcontainer.json conform to the spec; see p.Validate()

	standardizedJSON []byte // The raw contents of the target devcontainer.json, converted to standard JSON
}

// NewParser returns a Parser targeting a devcontainer.json via
// filepath. A few fields are initialized, and the returned Parser is
// ready to perform additional operations.
func NewParser(configPath string) Parser {
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		panic(err)
	}
	p := Parser{
		Filepath:      absConfigPath,
		IsValidConfig: false,
	}
	stdJSON, err := p.standardizeJSON()
	if err != nil {
		panic(err)
	}
	p.standardizedJSON = stdJSON
	return p
}

// Validate runs the contents of the target devcontainer.json against
// a snapshot of the devcontainer spec's official JSON Schema.
//
// A successful validation operation returns err == nil and sets
// p.IsValidConfig accordingly. Until after this is run, the value of
// p.IsValidConfig should not be considered definitive.
func (p *Parser) Validate() error {
	slog.Debug("initializing JSON schema validator")
	dcSchema, err := jsonschema.UnmarshalJSON(strings.NewReader(devcontainerJSONSchema))
	if err != nil {
		slog.Error("unable to unmarshal embedded JSON schema", "error", err)
		return err
	}
	c := jsonschema.NewCompiler()
	if err = c.AddResource(devcontainerJSONSchemaPath, dcSchema); err != nil {
		slog.Error("unable to add embedded JSON schema as resource", "error", err)
		return err
	}
	sch, err := c.Compile(devcontainerJSONSchemaPath)
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

// Parse the contents of the target devcontainer.json into a struct.
//
// Will refuse to parse unless the contents are determined to conform
// to the official JSON Schema spec.
//
// TODO: Add support for other parts of the spec. (Ongoing)
func (p *Parser) Parse() error {
	if !p.IsValidConfig {
		return errors.New("devcontainer.json flagged invalid")
	}

	slog.Debug("attempting to parse and unmarshal devcontainer.json")
	if err := json.Unmarshal(p.standardizedJSON, &p.Config); err != nil {
		slog.Error("failed to unmarshal JSON", "error", err, "path", p.Filepath)
		return err
	}

	if p.Config.RunArgs != nil {
		slog.Warn("devcontainer.json specifies runArgs that won't ever be used", "runArgs", p.Config.RunArgs)
	}

	slog.Debug("performing value normalization")
	if p.Config.Context == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		slog.Debug("no value given; using current working directory", "root/context", cwd)
		// Use the current working directory as context for builds if
		// none is given
		*p.Config.Context = cwd
	} else {
		// The value of context is relative to the devcontainer.json
		contextPath := filepath.Join(filepath.Dir(p.Filepath), *p.Config.Context)
		slog.Debug("converting value to absolute path", "root/context", *p.Config.Context, "actual", contextPath)
		*p.Config.Context = contextPath
	}

	if p.Config.DockerFile != nil {
		// Convert to a path usable for building images
		buildablePath, err := filepath.Rel(*p.Config.Context, filepath.Join(filepath.Dir(p.Filepath), *p.Config.DockerFile))
		if err != nil {
			slog.Error("unable to build relative path", "root/dockerFIle", *p.Config.DockerFile, "error", err)
			return err
		}
		slog.Debug("converting value to buildable path", "root/dockerFile", *p.Config.DockerFile, "actual", buildablePath)
		// ToSlash is necessary for usage on Windows
		*p.Config.DockerFile = filepath.ToSlash(buildablePath)
	}

	if p.Config.Init == nil {
		defInit := false
		p.Config.Init = &defInit
	}

	if p.Config.OverrideCommand == nil {
		defOverride := p.Config.DockerComposeFile != nil
		p.Config.OverrideCommand = &defOverride
	}

	if p.Config.Privileged == nil {
		defPrivileged := false
		p.Config.Privileged = &defPrivileged
	}

	if p.Config.ShutdownAction == nil {
		defShutdownAction := writ.ShutdownActionNone
		if p.Config.DockerComposeFile != nil {
			defShutdownAction = writ.StopCompose
		} else {
			defShutdownAction = writ.StopContainer
		}
		p.Config.ShutdownAction = &defShutdownAction
	}

	if p.Config.UpdateRemoteUserUID == nil {
		defUpdateRemoteUserUID := true
		// The spec states this defaults to true
		p.Config.UpdateRemoteUserUID = &defUpdateRemoteUserUID
	}

	// TODO: Investigate if "/workspace" actual is the default value
	// that's supposed to be used here.
	if p.Config.WorkspaceFolder == nil {
		defaultWorkspaceFolder := "/workspace"
		p.Config.WorkspaceFolder = &defaultWorkspaceFolder
		slog.Debug("no value given; using current default value", "root/workspaceFolder", *p.Config.WorkspaceFolder)
	}

	return nil
}

// Convert the contents of the target devcontainer.json, which could
// be JSONC, into standard JSON suitable for validation and parsing.
func (p *Parser) standardizeJSON() ([]byte, error) {
	slog.Debug("attempting to standardize devcontainer.json contents", "path", p.Filepath)
	file, err := os.Open(p.Filepath)
	if err != nil {
		slog.Error("failed to open devcontainer.json", "error", err, "path", p.Filepath)
		return nil, err
	}
	defer file.Close()

	fileInput, err := io.ReadAll(file)
	if err != nil {
		slog.Error("failed to read contents of devcontainer.json", "error", err, "path", p.Filepath)
		return nil, err
	}

	stdJSON, err := hujson.Standardize(fileInput)
	if err != nil {
		slog.Error("failed to standardize devcontainer.json contents", "error", err, "path", p.Filepath)
		return nil, err
	}

	return stdJSON, nil
}
