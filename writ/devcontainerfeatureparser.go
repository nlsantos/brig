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
	_ "embed"
	"encoding/json"
	"errors"
	"log/slog"
)

// devcontainerJSONSchema is the contents of the JSON schema against
// which devcontainer.json files are validated.
//
//go:embed specs/devContainerFeature.schema.json
var devcontainerFeatureJSONSchema string

// devcontainerJSONSchemaPath is the path used for the JSON schema
// when being added manually as resource for the validator; it allows
// the schema contents to be referenced by other resources later on.
const devcontainerFeatureJSONSchemaPath string = "devContainerFeature.schema.json"

type DevcontainerFeatureParser struct {
	Config DevcontainerFeatureConfig
	Parent *DevcontainerParser

	Parser
}

func NewDevcontainerFeatureParser(configPath string, parent *DevcontainerParser) (p *DevcontainerFeatureParser, err error) {
	parser, err := NewParser(configPath)
	if err != nil {
		return nil, err
	}
	parser.jsonSchema = devcontainerFeatureJSONSchema
	parser.jsonSchemaPath = devcontainerFeatureJSONSchemaPath
	return &DevcontainerFeatureParser{
		Parser: *parser,
		Parent: parent,
	}, nil
}

func (p *DevcontainerFeatureParser) Parse() error {
	if !p.IsValidConfig {
		return errors.New("devcontainer-feature.json flagged invalid")
	}

	slog.Debug("attempting to unmarshal and parse devcontainer-feature.json", "path", p.Filepath)
	if err := json.Unmarshal(p.standardizedJSON, &p.Config); err != nil {
		slog.Error("failed to unmarshal JSON", "path", p.Filepath, "error", err)
		return err
	}

	slog.Debug("configuration parsed", "config", p.Config)
	return nil
}
