package writ

import (
	"fmt"
	"io"
	"log/slog"
	"os"
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
	assert.False(t, p.IsValidConfig)
	if err := p.Parse(); err == nil {
		t.Fatal("parsed an invalid/unvalidated devcontainer.json")
	}
	// A devcontasiner.json needs to be validated before being parsed
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation")
	}
	assert.True(t, p.IsValidConfig)
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

	containerEnv := map[string]string{
		"APP_PATH": DefWorkspacePath,
		"SHELL":    "/bin/bash",
	}

	// Check fields against known values
	assert.Equal(t, "simple-ish devcontainer.json", *p.Config.Name, "fields not matching")
	assert.Equal(t, filepath.Join(filepath.Dir(p.Filepath), ".."), *p.Config.Context, "fields not matching")
	assert.Equal(t, "parse/Containerfile", *p.Config.DockerFile, "fields not matching")
	assert.Equal(t, containerEnv, p.Config.ContainerEnv, "fields not matching")
}

// TestParseForwardPorts parses a devcontainer.json that declares
// forwardPorts and validates that defaults port attributes are
// generated and applied
func TestParseForwardPorts(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p := NewParser(filepath.Join("testdata", "parse", "forward-ports.json"))
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation:", err)
	}
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing")
	}

	assert.NotNil(t, p.Config.ForwardPorts)
	assert.NotEmpty(t, p.Config.ForwardPorts)
	assert.NotNil(t, p.Config.PortsAttributes)

	for _, portAttribute := range p.Config.PortsAttributes {
		// This is defined in the devcontainer.json as a default value for ports
		assert.Equal(t, true, *portAttribute.ElevateIfNeeded)
	}
}

// TestParsePortsAttributes parses a devcontainer.json that declares
// forwardPorts *AND* portsAttributes and validates that explicit port
// attributes are able to override default values
func TestParsePortsAttributes(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p := NewParser(filepath.Join("testdata", "parse", "ports-attributes.json"))
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation:", err)
	}
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing")
	}

	assert.NotNil(t, p.Config.ForwardPorts)
	assert.NotEmpty(t, p.Config.ForwardPorts)
	assert.NotNil(t, p.Config.PortsAttributes)

	port8k, ok := p.Config.PortsAttributes["8000"]
	assert.Equal(t, true, ok)
	assert.EqualValues(t, "web port", *port8k.Label)
	assert.EqualValues(t, "tcp", *port8k.Protocol)
	assert.EqualValues(t, "notify", *port8k.OnAutoForward)
	assert.EqualValues(t, false, *port8k.RequireLocalPort)
	assert.EqualValues(t, false, *port8k.ElevateIfNeeded)

	port9k, ok := p.Config.PortsAttributes["9000"]
	assert.Equal(t, true, ok)
	assert.Empty(t, port9k.Label)
	assert.EqualValues(t, "tcp", *port9k.Protocol)
	assert.EqualValues(t, "notify", *port9k.OnAutoForward)
	assert.EqualValues(t, false, *port9k.RequireLocalPort)
	assert.EqualValues(t, false, *port9k.ElevateIfNeeded)

	portDb, ok := p.Config.PortsAttributes["db:5432"]
	assert.Equal(t, true, ok)
	assert.Empty(t, portDb.Label)
	assert.EqualValues(t, "tcp", *portDb.Protocol)
	assert.EqualValues(t, "notify", *portDb.OnAutoForward)
	assert.EqualValues(t, false, *portDb.RequireLocalPort)
	assert.EqualValues(t, true, *portDb.ElevateIfNeeded)

	portSdns, ok := p.Config.PortsAttributes["sdns:853"]
	assert.Equal(t, true, ok)
	assert.EqualValues(t, "secure DNS", *portSdns.Label)
	assert.EqualValues(t, "tcp", *portSdns.Protocol)
	assert.EqualValues(t, "notify", *portSdns.OnAutoForward)
	assert.EqualValues(t, false, *portSdns.RequireLocalPort)
	assert.EqualValues(t, true, *portSdns.ElevateIfNeeded)
}

// TestParseVarExpansion exercises writ's variable expansion.
func TestParseVarExpansion(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Set up the local env var table
	localEnvVars := map[string]string{
		"BRIG_TEST_VAR":          "Hello",
		"BRIG_TEST_MOUNT_SOURCE": "/brig/mount/source",
		"BRIG_TEST_MOUNT_TARGET": "/brig/mount/target",
	}
	for key, val := range localEnvVars {
		if err := os.Setenv(key, val); err != nil {
			t.Errorf("error setting up env var: %v", err)
		}
	}

	p := NewParser(filepath.Join("testdata", "parse", "variable-expansion.json"))
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation")
	}
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing")
	}

	containerEnv := map[string]string{
		// devcontainer spec vars
		"CONTAINER_WORKSPACE_FOLDER":          DefWorkspacePath,
		"CONTAINER_WORKSPACE_FOLDER_BASENAME": filepath.Base(DefWorkspacePath),
		"LOCAL_WORKSPACE_FOLDER":              *p.Config.Context,
		"LOCAL_WORKSPACE_FOLDER_BASENAME":     filepath.Base(*p.Config.Context),
		// Regular env vars
		"BRIG_TEST_VAR_INDIRECT":    localEnvVars["BRIG_TEST_VAR"],
		"BRIG_TEST_VAR_NONEXISTING": "",
		// Default value setting
		"BRIG_TEST_VAR_WITH_DEFAULT_STATIC":   "static",
		"BRIG_TEST_VAR_WITH_DEFAULT_INDIRECT": localEnvVars["BRIG_TEST_VAR"],
		"BRIG_TEST_VAR_SUB_EMPTY":             "",
		"BRIG_TEST_VAR_SUB_NOT_EMPTY":         "not empty",
		// Case conversion; lowercase conversion doesn't seem to be
		// supported at this time
		"BRIG_TEST_VAR_CASE_ALL_UPPER": "HELLO",
		// Variable length
		"BRIG_TEST_VAR_LENGTH": fmt.Sprintf("%d", len(localEnvVars["BRIG_TEST_VAR"])),
		// Variable offsets
		"BRIG_TEST_VAR_OFFSET_TWO":             localEnvVars["BRIG_TEST_VAR"][2:],
		"BRIG_TEST_VAR_OFFSET_TWO_TO_FOUR":     localEnvVars["BRIG_TEST_VAR"][2:4],
		"BRIG_TEST_VAR_OFFSET_NEG_FOUR":        localEnvVars["BRIG_TEST_VAR"][len(localEnvVars["BRIG_TEST_VAR"])-4:],
		"BRIG_TEST_VAR_OFFSET_NEG_FOUR_TO_TWO": localEnvVars["BRIG_TEST_VAR"][len(localEnvVars["BRIG_TEST_VAR"])-4 : 3],
		// Pattern substitution
		"BRIG_TEST_VAR_SUB_FIRST_L_TO_K": "Heklo",
		"BRIG_TEST_VAR_SUB_ALL_L_TO_K":   "Hekko",
		// Corresponding shell functionality not supported by shell.Expand()
		"BRIG_TEST_VAR_ASSIGNMENT":       "",
		"BRIG_TEST_VAR_ASSIGNMENT_CHECK": "",
		"BRIG_TEST_VAR_ERROR_ON_EMPTY":   "",
	}

	// Check fields against known values
	assert.Equal(t, *p.Config.Name, "devcontainer.json with variables", "fields not matching")
	assert.Equal(t, *p.Config.Context, filepath.Join(filepath.Dir(p.Filepath), ".."), "fields not matching")
	assert.Equal(t, *p.Config.DockerFile, "parse/Containerfile", "fields not matching")
	assert.Equal(t, p.Config.ContainerEnv, containerEnv, "fields not matching")

	for _, mount := range p.Config.Mounts {
		assert.Equal(t, mount.Mount.Source, localEnvVars["BRIG_TEST_MOUNT_SOURCE"])
		assert.Equal(t, mount.Mount.Target, localEnvVars["BRIG_TEST_MOUNT_TARGET"])
	}
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
