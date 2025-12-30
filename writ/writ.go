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
	"regexp"
	"strings"

	"dario.cat/mergo"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/tailscale/hujson"
	"mvdan.cc/sh/v3/shell"
)

// devcontainerJSONSchema is the contents of the JSON schema against
// which devcontainer.json files are validated.
//
//go:embed specs/devContainer.base.schema.json
var devcontainerJSONSchema string

// devcontainerJSONSchemaPath is the path used for the JSON schema
// when being added manually as resource for the validator; it allows
// the schema contents to be referenced by other resources later on.
const devcontainerJSONSchemaPath string = "devContainer.base.schema.json"

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

// A Parser contains metadata about a target devcontainer.jkson file,
// as well as the configuration for the intended devcontainer itself.
type Parser struct {
	Filepath      string             // Path to the target devcontainer.json
	Config        DevcontainerConfig // The parsed contents of the target devcontainer.json
	IsValidConfig bool               // Whether or not the contents of the devcontainer.json conform to the spec; see p.Validate()

	defaultValues    map[string]any // Default values of various fields keyed by their name
	standardizedJSON []byte         // The raw contents of the target devcontainer.json, converted to standard JSON
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
		defaultValues: make(map[string]any),
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

	if err := p.setDefaultValues(); err != nil {
		slog.Error("encountered an error while attempting to set default values", "error", err)
		return err
	}

	slog.Debug("attempting to unmarshal and parse devcontainer.json")
	if err := json.Unmarshal(p.standardizedJSON, &p.Config); err != nil {
		slog.Error("failed to unmarshal JSON", "path", p.Filepath, "error", err)
		return err
	}

	if p.Config.RunArgs != nil {
		slog.Warn("devcontainer.json uses runArgs, which is currently unsupported", "runArgs", p.Config.RunArgs)
	}

	if err := p.normalizeValues(); err != nil {
		slog.Error("encountered an error while attempting to normalize values", "error", err)
		return err
	}

	slog.Debug("configuration parsed", "config", p.Config)
	slog.Info("workspace folder", "path", *p.Config.WorkspaceFolder)

	return nil
}

// ExpandEnv is a thin wrapper around shell.Expand() that converts
// special devcontainer spec variables so they are more easily parsed
// like a regular shell variable.
//
// The devcontainer spec has special variable lookups that indicate
// scope (the `localEnv:`, `containerEnv:`, and the undocumented `env:`
// prefixes); unforunately, they also conflict with well-established
// shell parameter expansion rules.
//
// When parsing strings that could conceivably contain env vars using
// these prefixes, transform them to a form that lets them be passed
// to shell.Expand() while still keeping the other expansion
// capabilities.
func (p *Parser) ExpandEnv(v string) string {
	// These two prefixes are easy since they're just local var
	// lookups, so they can just be discarded
	localEnvPrefixes := regexp.MustCompile(`(\$\{)(env|localEnv):`)
	v = localEnvPrefixes.ReplaceAllString(v, "$1")
	// This is a little trickier. It's highly unlikely, but entirely
	// *possible* that, after swapping in the prefix, the resulting
	// variable name ends up clashing with an existing env var. In
	// that case, that env var will be shadowed by an env var that
	// doesn't have the prefix.
	envPrefixes := regexp.MustCompile(`(\$\{containerEnv):`)
	v = envPrefixes.ReplaceAllString(v, "${1}__")

	retval, err := shell.Expand(v, p.expandEnv)
	if err != nil {
		slog.Debug("error expanding env var", "var", v, "error", err)
	}
	return retval
}

// expandEnv is the variable "storage" that provides values to
// shell.Expand() when called by it.
//
// Expects v to be the string name of an environment variable to look
// up. If it is one of the specially named variables in the
// devcontainer spec, it returns the expected special
// value. Otherwise, performs a lookup for an actual env var with the
// given name, and returns its value if it exists. If either lookups
// fail, returns an empty string.
func (p *Parser) expandEnv(v string) string {
	switch {
	case v == "containerWorkspaceFolder":
		return DefWorkspacePath
	case v == "containerWorkspaceFolderBasename":
		return filepath.Base(DefWorkspacePath)
	case v == "localWorkspaceFolder":
		return *p.Config.Context
	case v == "localWorkspaceFolderBasename":
		return filepath.Base(*p.Config.Context)
	case strings.HasPrefix(v, "containerEnv__"):
		envKey := strings.SplitN(v, "__", 2)
		slog.Error("container env var looks are not yet implemented", "var", envKey[1])
		return ""
	default:
		return os.Getenv(v)
	}
}

// normalizeValues goes through a devcontainer.json's values and
// massages them as needed.
//
// This may involve setting default values, converting relative paths
// to absolute paths (or the reverse), turning raw values into
// easier-to-use ones, etc.
func (p *Parser) normalizeValues() error {
	slog.Debug("performing value normalization")

	if !filepath.IsAbs(*p.Config.Context) {
		// The value of context is relative (if it is relative) to the devcontainer.json
		contextPath := filepath.Join(filepath.Dir(p.Filepath), *p.Config.Context)
		slog.Debug("converting value to absolute path", "root/context", *p.Config.Context, "actual", contextPath)
		*p.Config.Context = contextPath
	}

	if p.Config.DockerFile != nil {
		// Convert to a path usable for building images
		buildablePath, err := filepath.Rel(*p.Config.Context, filepath.Join(filepath.Dir(p.Filepath), *p.Config.DockerFile))
		if err != nil {
			slog.Error("unable to build relative path", "root/dockerFile", *p.Config.DockerFile, "error", err)
			return err
		}
		slog.Debug("converting value to buildable path", "root/dockerFile", *p.Config.DockerFile, "actual", buildablePath)
		// ToSlash is necessary for usage on Windows
		*p.Config.DockerFile = filepath.ToSlash(buildablePath)
	}

	if p.Config.DockerComposeFile != nil {
		var composeFiles []string
		for _, compose := range *p.Config.DockerComposeFile {
			buildablePath, err := filepath.Rel(*p.Config.Context, filepath.Join(filepath.Dir(p.Filepath), compose))
			if err != nil {
				slog.Error("unable to build relative path", "root/dockerComposeFile[]", compose, "error", err)
				return err
			}
			slog.Debug("converting value to buildable path", "root/dockerComposeFile", compose, "actual", buildablePath)
			// ToSlash is necessary for usage on Windows
			composeFiles = append(composeFiles, filepath.ToSlash(buildablePath))
		}
		*p.Config.DockerComposeFile = composeFiles
	}

	if len(p.Config.ForwardPorts) > 0 {
		slog.Debug("sorting out forwardPorts")
		val := p.defaultValues["otherPortsAttributes"]
		if defOtherPortsAttributes, ok := val.(PortAttributes); ok {
			if err := mergo.Merge(p.Config.OtherPortsAttributes, defOtherPortsAttributes); err != nil {
				slog.Error("unable to merge default values for otherPortsAttributes", "error", err)
				return err
			}
		}

		for _, portIdx := range p.Config.ForwardPorts {
			portAttributes := p.Config.PortsAttributes[portIdx]
			if err := mergo.Merge(&portAttributes, p.Config.OtherPortsAttributes); err != nil {
				slog.Error("unable to merge default values for portsAttributes", "port", portIdx, "error", err)
				return err
			}
			p.Config.PortsAttributes[portIdx] = portAttributes
		}
	}

	if p.Config.ContainerEnv != nil {
		slog.Debug("expanding variables", "section", "containerEnv")
		for key, val := range p.Config.ContainerEnv {
			p.Config.ContainerEnv[key] = p.ExpandEnv(val)
		}
	}

	if p.Config.Mounts != nil {
		slog.Debug("expanding variables", "section", "mounts")
		for _, mount := range p.Config.Mounts {
			mount.Mount.Source = p.ExpandEnv(mount.Mount.Source)
			mount.Mount.Target = p.ExpandEnv(mount.Mount.Target)
		}
	}

	// Defaults to true for when using an image Dockerfile and false
	// when referencing a Docker Compose file.
	if p.Config.OverrideCommand == nil {
		defOverride := p.Config.DockerComposeFile == nil
		p.Config.OverrideCommand = &defOverride
	}

	// Basically, this only gets set to "none" if done so explcitly.
	if p.Config.ShutdownAction == nil {
		var defShutdownAction ShutdownAction
		if p.Config.DockerComposeFile == nil {
			defShutdownAction = StopContainer
		} else {
			defShutdownAction = StopCompose
		}
		p.Config.ShutdownAction = &defShutdownAction
	}

	return nil
}

// setDefaultValues assigns default values to certain fields.
//
// This function only deals with values that can be computed without
// referencing other values that need to be computed (beyond, say,
// simple comparisons); for those, refer to normalizeValues().
func (p *Parser) setDefaultValues() error {
	slog.Debug("setting up default values")

	defFalse := false
	defTrue := true
	defForwardNotify := Notify
	// This isn't one of the explicitly defined values for this field,
	// but the spec states that if this field is unset,
	// imeplementations are expected to behave as though it's set to
	// "tcp"
	defProtocol := Protocol("tcp")
	defWorkspacePath := DefWorkspacePath

	// Use the current working directory as context for builds if
	// none is given
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	p.Config.Context = &cwd

	defPortAttributes := PortAttributes{
		Label:            nil,
		Protocol:         &defProtocol,
		OnAutoForward:    &defForwardNotify,
		RequireLocalPort: &defFalse,
		ElevateIfNeeded:  &defFalse,
	}
	p.defaultValues["otherPortsAttributes"] = defPortAttributes

	p.Config.Init = &defFalse
	p.Config.OtherPortsAttributes = &defPortAttributes
	p.Config.PortsAttributes = map[string]PortAttributes{}
	p.Config.Privileged = &defFalse
	p.Config.UpdateRemoteUserUID = &defTrue
	p.Config.WorkspaceFolder = &defWorkspacePath

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
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("could not close JSON file while standardizing", "error", err)
		}
	}()

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
