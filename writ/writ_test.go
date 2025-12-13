package writ

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParse checks and illustrates the exepcted flow of parsing;
// there are additional tests that exercise the parsing functionality
func TestParse(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p := NewParser(filepath.Join("testdata", "parse", "simple-devcontainer.json"))
	// Parsing an unvalidated file should fail
	if err := p.Parse(); err == nil {
		t.Fatal("parsed an invalid/unvalidated devcontainer.json")
	}
	// A devcontasiner.json needs to be validated before being parsed
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation")
	}
	// This should now work
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing")
	}
}

// TestParseSimple parses a simple devcontainer.json and checks that
// the unmarshalled values match as expected
func TestParseSimple(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p := NewParser(filepath.Join("testdata", "parse", "simple-devcontainer.json"))
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation")
	}
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing")
	}

	containerEnv := make(map[string]string)
	// TODO: Implement interpolation for variables
	containerEnv["APP_PATH"] = "${containerWorkspaceFolder}"
	containerEnv["SHELL"] = "/bin/bash"

	// Check fields against known values
	assert.Equal(t, *p.Config.Name, "simple-ish devcontainer.json", "fields not matching")
	assert.Equal(t, *p.Config.Context, "..", "fields not matching")
	assert.Equal(t, *p.Config.DockerFile, "./Containerfile", "fields not matching")
	assert.Equal(t, p.Config.ContainerEnv, containerEnv, "fields not matching")
}

// TestValidate attempts validation of known valid and invalid samples
// of devcontainer.json files.
func TestValidate(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	pathsValidSamples, err := filepath.Glob(filepath.Join("testdata", "validate", "valid-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pathsValidSamples) < 1 {
		t.Error("unable to find valid devcontainer.json samples")
	} else {
		t.Run("AgainstValidSamples", func(t *testing.T) {
			for _, path := range pathsValidSamples {
				p := NewParser(path)
				if err = p.Validate(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}

	pathsInvalidSamples, err := filepath.Glob(filepath.Join("testdata", "validate", "invalid-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pathsInvalidSamples) < 1 {
		t.Error("unable to find invalid devcontainer.json samples")
	} else {
		t.Run("AgainstInvalidSamples", func(t *testing.T) {
			for _, path := range pathsInvalidSamples {
				p := NewParser(path)
				if err = p.Validate(); err == nil {
					t.Fatal("known-invalid sample passed validation: ", path)
				}
			}
		})
	}
}
