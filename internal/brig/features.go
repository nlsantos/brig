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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/codeclysm/extract"
	"github.com/nlsantos/brig/writ"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
)

const FeatureArtifactMediaType string = "application/vnd.oci.image.manifest.v1+json"
const FeatureLayerMediaType string = "application/vnd.devcontainers.layer.v1+tar"

func (cmd *Command) ParseFeaturesConfig(ctx context.Context, p *writ.DevcontainerParser, featureMap writ.FeatureMap) (err error) {
	for featureID, featureMap := range featureMap {
		slog.Debug("initializing configuration for feature", "feature", featureID)
		featurePath, ok := cmd.featurePathLookup[featureID]
		if !ok {
			return fmt.Errorf("feature unavailable for parsing: %s", featurePath)
		}

		featureParser, err := writ.NewDevcontainerFeatureParser(filepath.Join(featurePath, "devcontainer-feature.json"), p)
		if err != nil {
			return err
		}
		if err = featureParser.Validate(); err != nil {
			return nil
		}
		if err = featureParser.Parse(); err != nil {
			return nil
		}

		for key, val := range featureMap {
			if err = featureParser.SetOption(key, val); err != nil {
				return nil
			}
		}

		cmd.featureParsersLookup[featureID] = featureParser
	}
	return nil
}

// PrepareFeaturesData retrieves each Feature's component files
// (downloading them from remote endpoints if necessary, then caching
// them for future use) and makes the parsed config available as
// values in a lookup table.
func (cmd *Command) PrepareFeaturesData(ctx context.Context, featureMap writ.FeatureMap, contextPath string) (err error) {
	for featureID := range featureMap {
		slog.Debug("attempting to pull feature metadata", "feature", featureID)
		var featurePath string
		switch {
		case strings.HasPrefix(featureID, "/"):
			// https://containers.dev/implementors/features-distribution/#addendum-locally-referenced
			return fmt.Errorf("locally-stored features may not be referenced by an absolute path: %s", featureID)

		// Features available on the local filesystem aren't
		// redirected to the cache, unlike HTTPS-hosted tarballs and
		// OCI artifacts, but are instead used as-is.
		case strings.HasPrefix(featureID, "./"):
			if featurePath, err = filepath.Abs(filepath.Join(filepath.Dir(contextPath), featureID)); err != nil {
				return err
			}
			slog.Debug("referencing a locally-stored feature", "path", featurePath)
			if _, err = os.Stat(featurePath); errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("referenced a locally-stored feature that doesn't exist: %s", featurePath)
			}

		case strings.HasPrefix(featureID, "https://"):
			if featurePath, err = cmd.prepareFeatureDataURI(ctx, featureID); err != nil {
				return err
			}

		default:
			if featurePath, err = cmd.prepareFeatureDataArtifact(ctx, featureID); err != nil {
				return err
			}
		}

		cmd.featurePathLookup[featureID] = featurePath
	}
	return nil
}

func (cmd *Command) prepareFeatureDataArtifact(ctx context.Context, ref string) (path string, err error) {
	slog.Debug("attempting to pull feature OCI artifact", "ref", ref)
	cacheDir, err := cmd.getCacheDirectory()
	if err != nil {
		slog.Error("encountered an error while attempting to get cache directory", "error", err)
	}

	repo, err := remote.NewRepository(ref)
	if err != nil {
		return "", err
	}

	slog.Debug("attempting to resolve reference to an OCI artifact")
	description, err := repo.Resolve(ctx, repo.Reference.Reference)
	if err != nil {
		return "", err
	}

	slog.Debug("retrieved metadata for an OCI artifact", "digest", string(description.Digest))
	// Check if this is already present in the cache; we use the
	// digest reported by the server as an ID (i.e., the directory
	// name)
	splitDigest := strings.Split(string(description.Digest), ":")
	digest := splitDigest[len(splitDigest)-1]
	possibleCachedArtifactPath, err := filepath.Abs(filepath.Join(cacheDir, digest))
	if err != nil {
		return "", err
	}
	slog.Debug("checking if artifact exists in cache", "path", possibleCachedArtifactPath)
	if _, err := os.Stat(possibleCachedArtifactPath); err == nil {
		// Should there be additional checks to ensure that the cached
		// copy is valid?
		slog.Debug("returning path of cached artifact copy", "path", possibleCachedArtifactPath)
		return possibleCachedArtifactPath, nil
	}

	if description.MediaType != FeatureArtifactMediaType {
		slog.Error("feature URI resolved to an unsupported media type", "mime", description.MediaType)
		return "", err
	}

	slog.Debug("retrieving OCI artifact manifest")
	_, manifestContent, err := oras.FetchBytes(ctx, repo, ref, oras.DefaultFetchBytesOptions)
	if err != nil {
		return "", err
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestContent, &manifest); err != nil {
		return "", err
	}
	slog.Debug("retrieved manifest; iterating over layers", "mime", manifest.MediaType, "layerCount", len(manifest.Layers))
	for _, layer := range manifest.Layers {
		if layer.MediaType != FeatureLayerMediaType {
			continue
		}
		slog.Debug("found layer with the target media type; extracting to cache", "path", possibleCachedArtifactPath)
		if _, err := os.Stat(possibleCachedArtifactPath); errors.Is(err, fs.ErrNotExist) {
			if err = os.Mkdir(possibleCachedArtifactPath, fs.ModeDir|0755); err != nil {
				return "", err
			}
		}

		layerBytes, err := content.FetchAll(ctx, repo, layer)
		if err != nil {
			return "", err
		}
		if err = extract.Tar(ctx, bytes.NewBuffer(layerBytes), possibleCachedArtifactPath, nil); err != nil {
			return "", err
		}

		return possibleCachedArtifactPath, nil
	}

	return "", fmt.Errorf("referenced OCI artifact didn't contain a usable layer")
}

func (cmd *Command) prepareFeatureDataURI(_ context.Context, uri string) (path string, err error) {
	slog.Debug("attempting to pull feature tarball", "uri", uri)
	_, err = cmd.getCacheDirectory()
	if err != nil {
		slog.Error("encountered an error while attempting to get cache directory", "error", err)
	}
	return "", nil
}
