/*
   brig: The lightweight, native Go CLI for devcontainers
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

// Package brig houses a CLI tool for working with devcontainer.json
package brig

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/nlsantos/brig/writ"
)

// PrepareFeaturesData retrieves each Feature's metadata (downloading
// it from remote endpoints as necessary, storing them in a temporary
// directory with a randomly-generated name) and makes that info
// available as values in a lookup table.
//
// Based on the wording of the devcontainer spec
// (https://containers.dev/implementors/features/#referencing-a-feature),
// it would seem that the resolution order needs to be:
//
//	OCI artifact -> HTTPS-hosted tarball -> local directory
//
// However, as it's more convenient this way, brig does:
//
//	HTTPS-hosted tarball -> local directory -> OCI artifact
//
// i.e., it's possible to shadow OCI artifacts by creating local
// directories. This is considered a feature, as this allows brig to be
// used without a network connection.
func (cmd *Command) PrepareFeaturesData(ctx context.Context, p *writ.DevcontainerParser) error {
	for featureID := range p.Config.Features {
		if strings.HasPrefix(featureID, "https://") {
			path, err := cmd.prepareFeatureDataURI(ctx, featureID)
			if err != nil {
				return err
			}
			cmd.featuresLookup[featureID] = path
			continue
		}

		// Features available on the local filesystem aren't
		// redirected to the cache, unlike HTTPS-hosted tarballs and
		// OCI artifacts, but are instead used as-is.
		if absPath, err := filepath.Abs(filepath.Join(filepath.Dir(p.Filepath), featureID)); err == nil {
			slog.Debug("referencing a locally-stored feature", "path", absPath)
			if _, err := os.Stat(absPath); !errors.Is(err, fs.ErrNotExist) {
				cmd.featuresLookup[featureID] = &absPath
				continue
			}
		}

		path, err := cmd.prepareFeatureDataArtifact(ctx, featureID)
		if err != nil {
			return err
		}
		cmd.featuresLookup[featureID] = path
	}
	return nil
}

func (cmd *Command) prepareFeatureDataArtifact(_ context.Context, ref string) (path *string, err error) {
	slog.Debug("attempting to pull feature OCI artifact", "ref", ref)
	_, err = cmd.getCacheDirectory()
	if err != nil {
		slog.Error("encountered an error while attempting to get cache directory", "error", err)
	}
	spew.Dump(ref)
	return nil, err
}

func (cmd *Command) prepareFeatureDataURI(_ context.Context, uri string) (path *string, err error) {
	slog.Debug("attempting to pull feature tarball", "uri", uri)
	cacheDir, err := cmd.getCacheDirectory()
	if err != nil {
		slog.Error("encountered an error while attempting to get cache directory", "error", err)
	}
	spew.Dump(cacheDir)
	return nil, nil
}
