package writ

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseDevcontainerFeature checks and illustrates the exepcted flow of
// parsing; it also checks that the default values for fields are set
// correctly.
func TestParseDevcontainerFeature(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	parent, err := NewDevcontainerParser(filepath.Join("testdata", "parse", "devcontainer-feature", "_parent.json"))
	assert.Nil(t, err)

	p, err := NewDevcontainerFeatureParser(filepath.Join("testdata", "parse", "devcontainer-feature", "simple-devcontainer-feature.json"), parent)
	assert.Nil(t, err)
	// Parsing an unvalidated file should fail
	assert.False(t, p.IsValidConfig)
	if err := p.Parse(); err == nil {
		t.Fatal("parsed an invalid/unvalidated devcontainer-feature.json", "error", err)
	}
	// A devcontasiner.json needs to be validated before being parsed
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer-feature.json expected to be valid failed validation", "error", err)
	}
	assert.True(t, p.IsValidConfig)
	// This should now work
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer-feature.json expected to be valid failed parsing", "error", err)
	}

	assert.EqualValues(t, "feature", p.Config.ID)
	assert.EqualValues(t, "1.0.0", p.Config.Version)
	assert.EqualValues(t, "feature", *p.Config.Name)
	assert.EqualValues(t, "https://example.com", *p.Config.DocumentationURL)
	assert.EqualValues(t, "feature description", *p.Config.Description)
	assert.NotEmpty(t, p.Config.Options)

	assert.EqualValues(t, FeatureOptionTypeBoolean, p.Config.Options["ppa"].Type)
	assert.True(t, *p.Config.Options["ppa"].Default.Bool)
	assert.Nil(t, p.Config.Options["ppa"].Default.String)
	assert.EqualValues(t, "bool option description", *p.Config.Options["ppa"].Description)

	assert.EqualValues(t, FeatureOptionTypeString, p.Config.Options["version"].Type)
	assert.EqualValues(t, []string{"latest", "system", "os-provided"}, p.Config.Options["version"].Proposals)
	assert.Nil(t, p.Config.Options["version"].Default.Bool)
	assert.EqualValues(t, "os-provided", *p.Config.Options["version"].Default.String)
	assert.EqualValues(t, "string option description", *p.Config.Options["version"].Description)

	assert.EqualValues(t, []string{"ghcr.io/devcontainers/features/common-utils"}, p.Config.InstallsAfter)

	// We don't particularly care about the customizations field
}
