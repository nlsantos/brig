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
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gocarina/gocsv"
)

type ArtifactDigestEntry struct {
	FeatureID string `csv:"feature_id"`
	Digest    string `csv:"digest"`
}

type ArtifactDigest struct {
	Entries map[string]*ArtifactDigestEntry
}

func (cmd *Command) LoadArtifactDigest() error {
	if cmd.featureArtifactsDigests != nil {
		return nil
	}

	slog.Debug("loading artifact digest lookup table")
	cacheDir, err := cmd.getCacheDirectory()
	if err != nil {
		slog.Error("encountered an error while attempting to get cache directory", "error", err)
		return err
	}

	digestsTablePath := filepath.Join(cacheDir, "digests.csv")
	digestsTable, err := os.OpenFile(digestsTablePath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer digestsTable.Close()

	digests := []*ArtifactDigestEntry{}
	slog.Debug("attempting to unmarshal digests table")
	if err := gocsv.UnmarshalFile(digestsTable, &digests); err != nil && !errors.Is(err, gocsv.ErrEmptyCSVFile) {
		return err
	} else {
		// Initialize the struct manually
		cmd.featureArtifactsDigests = &ArtifactDigest{
			Entries: make(map[string]*ArtifactDigestEntry),
		}
	}
	slog.Debug("digests table successful unmarshalled")
	slog.Debug("artifact digest entries loaded", "count", len(digests))

	for _, digest := range digests {
		cmd.featureArtifactsDigests.Entries[digest.FeatureID] = digest
	}

	return nil
}

func (cmd *Command) SaveArtifactDigest() error {
	if cmd.featureArtifactsDigests == nil || len(cmd.featureArtifactsDigests.Entries) == 0 {
		return nil
	}

	slog.Debug("saving artifact digest lookup table")
	cacheDir, err := cmd.getCacheDirectory()
	if err != nil {
		slog.Error("encountered an error while attempting to get cache directory", "error", err)
		return err
	}

	digestsTablePath := filepath.Join(cacheDir, "digests.csv")
	digestsTable, err := os.OpenFile(digestsTablePath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}

	digests := []*ArtifactDigestEntry{}
	for _, digestEntry := range cmd.featureArtifactsDigests.Entries {
		digests = append(digests, digestEntry)
	}
	slog.Debug("artifact digest entries to be marshalled", "count", len(digests))
	slog.Debug("attempting to marshal digests table")
	if err = gocsv.MarshalFile(&digests, digestsTable); err != nil {
		return err
	}
	slog.Debug("digests table successfully marshalled")

	return nil
}
