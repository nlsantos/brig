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
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/codeclysm/extract/v4"
	"github.com/heimdalr/dag"
	"github.com/nlsantos/brig/writ"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
)

const FeatureArtifactMediaType string = "application/vnd.oci.image.manifest.v1+json"
const FeatureLayerMediaType string = "application/vnd.devcontainers.layer.v1+tar"

// BuildFeaturesInstallationGraph iterates over a devcontainer's
// Features and builds a directed acyclic graph that can be used to
// guide Features' installation order.
func (cmd *Command) BuildFeaturesInstallationGraph() (installDAG *dag.DAG, err error) {
	installDAG = dag.NewDAG()
	for featureID, featureParser := range cmd.featureParsersLookup {
		vertexID := featureID
		if !strings.HasPrefix(featureID, "https://") {
			// Remove version tags from feature IDs
			featureIDAndTag := strings.Split(featureID, ":")
			vertexID = featureIDAndTag[0]
		}
		if err := installDAG.AddVertexByID(vertexID, featureParser); err != nil {
			return nil, err
		}
	}

	// As of this writing, I'm yet to encounter an official feature
	// that actually utilizes the dependsOn field.
	for featureID, featureParser := range cmd.featureParsersLookup {
		for dependencyID := range featureParser.Config.DependsOn {
			edgeID := dependencyID
			if !strings.HasPrefix(dependencyID, "https://") {
				// Remove version tags from dependency IDs
				dependencyIDAndTag := strings.Split(featureID, ":")
				edgeID = dependencyIDAndTag[0]
			}
			installDAG.AddEdge(edgeID, featureID)
		}
	}

	// installsAfter entries are *soft* dependencies; if they're not
	// specifically declared in dependsOn, they may not even be
	// installed.
	//
	// https://containers.dev/implementors/features/#installsAfter
	for featureID, featureParser := range cmd.featureParsersLookup {
		for _, dependency := range featureParser.Config.InstallsAfter {
			if _, err = installDAG.GetVertex(dependency); err != nil {
				continue
			}
			installDAG.AddEdge(dependency, featureID)
		}
	}
	return installDAG, nil
}

// CopyFeaturesToContextDirectory iterates over a devcontainer's
// Features and copies their files from the cache directory into the
// devcontainer's context directory (an actual context directory if
// specified; otherwise, the directory where the devcontainer.json
// file resides in).
//
// This is necessary so the OCI build process can be rooted in a sane
// path and limited to the codebase it's working with.
//
// Returns the base directory within the context directory where the
// Features' files reside in as subdirectories.
func (cmd *Command) CopyFeaturesToContextDirectory(ctxPath string) (featuresBasePath string, err error) {
	// Create a single directory into which we copy features files
	if featuresBasePath, err = os.MkdirTemp(ctxPath, ".features-*"); err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(featuresBasePath)
		}
	}()
	// This will contain paths *within* the context directory that
	// will eventually be incorporated into the OCI image
	remoteFeaturePathLookup := make(map[string]string)
	for featureID, cachedFeaturePath := range cmd.featurePathLookup {
		// Create a tempdir to store feature files in; this gets
		// around possibly dealing with invalid path names if they're
		// based on feature references
		featurePath, err := os.MkdirTemp(featuresBasePath, "feature-*")
		if err != nil {
			return "", err
		}
		if err := os.CopyFS(featurePath, os.DirFS(cachedFeaturePath)); err != nil {
			return "", err
		}
		remoteFeaturePathLookup[featureID] = featurePath
	}
	// Overwrite previously set lookup table
	cmd.featurePathLookup = remoteFeaturePathLookup
	return featuresBasePath, nil
}

// GenerateContainerfileWithFeatures programmatically creates a
// custom, ephemeral Containerfile to be used in an OCI build process
// that ensures Features' files are incorporated into the resulting
// OCI image.
func (cmd *Command) GenerateContainerfileWithFeatures(ctxPath string, baseImage string) (containerfilePath string, err error) {
	containerfile, err := os.CreateTemp(ctxPath, fmt.Sprintf(".%s.Containerfile.*", cmd.appName))
	if err != nil {
		return "", err
	}
	defer containerfile.Close()

	remoteFeaturePathLookup := make(map[string]string)
	containerfile.WriteString(fmt.Sprintf("FROM %s\n", baseImage))
	for featureID, featurePath := range cmd.featurePathLookup {
		relFeaturePath, err := filepath.Rel(ctxPath, featurePath)
		if err != nil {
			return "", err
		}

		remotePath := fmt.Sprintf("/devcontainer-features/%d", rand.Int())
		remoteConfigPath := fmt.Sprintf("%s/devcontainer-feature.json", remotePath)

		remoteFeaturePathLookup[featureID] = remotePath
		// Massage feature parser to the path within the OCI image for
		// later execution
		cmd.featureParsersLookup[featureID].Filepath = remoteConfigPath
		containerfile.WriteString(fmt.Sprintf("COPY \"%s/*\" \"%s/\"\n", relFeaturePath, remotePath))
	}
	// Overwrite previously set lookup table
	cmd.featurePathLookup = remoteFeaturePathLookup
	containerfilePath = containerfile.Name()
	return containerfilePath, err
}

// ParseFeaturesConfig instantiates a writ.DevcontainerFeatureParser
// for each Feature a devcontainer references and stores it for later
// use by Command.
//
// It also instantiates a corresponding parser for every dependency
// referenced in the dependsOn field of a Feature configuration.
func (cmd *Command) ParseFeaturesConfig(ctx context.Context, p *writ.DevcontainerParser, featureMap writ.FeatureMap) (err error) {
	for featureID, featureMap := range featureMap {
		slog.Debug("initializing configuration for feature", "feature", featureID)
		featurePath, ok := cmd.featurePathLookup[featureID]
		if !ok {
			return fmt.Errorf("feature unavailable for parsing: %s", featurePath)
		}

		if _, ok := cmd.featureParsersLookup[featureID]; ok {
			slog.Debug("feature already parsed; skipping", "featureID", featureID)
			return nil
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
			if err = featureParser.SetOption(key, &val); err != nil {
				return err
			}
		}

		if err = cmd.PrepareFeaturesData(ctx, featureParser.Config.DependsOn, p.Filepath); err != nil {
			return err
		}
		if err = cmd.ParseFeaturesConfig(ctx, p, featureParser.Config.DependsOn); err != nil {
			return err
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
			if err = cmd.LoadArtifactDigest(); err != nil {
				return err
			}

			if featurePath, err = cmd.prepareFeatureDataArtifact(ctx, featureID); err != nil {
				return err
			}
		}

		cmd.featurePathLookup[featureID] = featurePath
	}
	return nil
}

// prepareFeatureDataArtifact handles retrieving Features that are
// distributed as OCI artifacts accessible via the reference `ref`.
//
// Currently only supports publicly-accessible OCI registries.
func (cmd *Command) prepareFeatureDataArtifact(ctx context.Context, ref string) (path string, err error) {
	slog.Debug("attempting to pull feature OCI artifact", "ref", ref)
	cacheDir, err := cmd.getCacheDirectory()
	if err != nil {
		slog.Error("encountered an error while attempting to get cache directory", "error", err)
		return "", err
	}

	cacheKeyComponents := []string{cacheDir}
	cacheKeyComponents = append(cacheKeyComponents, strings.Split(ref, ":")...)
	// cacheKey is the subdirectory within the root cache directory
	// where the contents of the OCI artifact are going to be stored
	cacheKey := filepath.Join(cacheKeyComponents...)

	_, err = os.Stat(cacheKey)
	cachedCopyExists := err == nil

	repo, err := remote.NewRepository(ref)
	if err != nil {
		return "", err
	}

	slog.Debug("attempting to resolve reference to an OCI artifact")
	description, err := repo.Resolve(ctx, repo.Reference.Reference)
	if err != nil {
		if cachedCopyExists {
			// If the OCI artifact is already cached, this *could* be
			// a recoverable situation, so return the cached path
			// instead of conking out.
			//
			// The only caveat is that we aren't able to validate that
			// the digests match, so the cache might be stale
			slog.Warn("resolving OCI reference returned an error but a cached (possibly stale) copy already exists", "error", err)
			return cacheKey, nil
		}
		return "", err
	}

	slog.Debug("retrieved metadata for an OCI artifact", "digest", string(description.Digest))
	digestTableEntry, ok := cmd.featureArtifactsDigests.Entries[ref]
	if ok && cachedCopyExists {
		if digestTableEntry.Digest == string(description.Digest) {
			slog.Info("digest matches cached copy", "reference", ref, "digest", digestTableEntry.Digest)
			return cacheKey, nil
		}
		slog.Info(
			"cached copy exists but digests don't match",
			"reference", ref,
			"localDigest", digestTableEntry.Digest,
			"remoteDigest", string(description.Digest),
		)
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
		slog.Debug("found layer with the target media type; extracting to cache", "path", cacheKey)
		if !cachedCopyExists {
			if err = os.MkdirAll(cacheKey, fs.ModeDir|0755); err != nil {
				return "", err
			}
		}

		layerBytes, err := content.FetchAll(ctx, repo, layer)
		if err != nil {
			return "", err
		}
		if err = extract.Tar(ctx, bytes.NewBuffer(layerBytes), cacheKey, nil); err != nil {
			return "", err
		}

		// Store the metadata for later marshalling
		cmd.featureArtifactsDigests.Entries[ref] = &ArtifactDigestEntry{
			FeatureID: ref,
			Digest:    string(description.Digest),
		}

		return cacheKey, nil
	}

	return "", fmt.Errorf("referenced OCI artifact didn't contain a usable layer")
}

// prepareFeatureDataURI handles Features distributed as tarballs via
// regular HTTPS endpoints.
//
// Currently a stub.
func (cmd *Command) prepareFeatureDataURI(_ context.Context, uri string) (path string, err error) {
	//slog.Debug("attempting to pull feature tarball", "uri", uri)
	slog.Error("SUPPORT FOR HTTPS-HOSTED FEATURE TARBALLS IS CURRENTLY UNIMPLEMENTED")
	_, err = cmd.getCacheDirectory()
	if err != nil {
		slog.Error("encountered an error while attempting to get cache directory", "error", err)
	}
	return "", nil
}
