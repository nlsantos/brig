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

// TestParseDevcontainer checks and illustrates the exepcted flow of
// parsing; it also checks that the default values for fields are set
// correctly.
func TestParseDevcontainer(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p, err := NewDevcontainerParser(filepath.Join("testdata", "parse", "devcontainer", "simple-devcontainer.json"))
	assert.Nil(t, err)
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

	// Check setting default values
	assert.True(t, *p.Config.OverrideCommand) // We're not using a Composer recipe
	assert.Equal(t, DefWorkspacePath, *p.Config.WorkspaceFolder)

	assert.Empty(t, p.Config.ForwardPorts)
	assert.Empty(t, p.Config.PortsAttributes)

	assert.Empty(t, p.Config.ContainerEnv)
	assert.Empty(t, p.Config.RemoteEnv)

	assert.Nil(t, p.Config.ContainerUser)
	assert.True(t, *p.Config.UpdateRemoteUserUID)

	assert.EqualValues(t, ShutdownActionStopContainer, *p.Config.ShutdownAction)
	assert.False(t, *p.Config.Init)
	assert.False(t, *p.Config.Privileged)

	assert.Empty(t, p.Config.CapAdd)
	assert.Empty(t, p.Config.SecurityOpt)

	assert.Empty(t, p.Config.Mounts)
	assert.Empty(t, p.Config.Features)
	assert.Empty(t, p.Config.OverrideFeatureInstallOrder)
	assert.Empty(t, p.Config.Customizations)
}

// TestParseDevcontainerAppPortInt parses a devcontainer.json with an
// appPort that consists of a single integer and checks that the
// unmarshalled values match as expected
func TestParseDevcontainerAppPortInt(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p, err := NewDevcontainerParser(filepath.Join("testdata", "parse", "devcontainer", "appport-single-int.json"))
	assert.Nil(t, err)
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation")
	}
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing")
	}

	appPort := []string{"8000"}
	assert.EqualValues(t, &appPort, p.Config.AppPort)
}

// TestParseDevcontainerAppPortMulti parses a devcontainer.json with
// an appPort that consists of integers and strings, and checks that
// the unmarshalled values match as expected
func TestParseDevcontainerAppPortMulti(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p, err := NewDevcontainerParser(filepath.Join("testdata", "parse", "devcontainer", "appport-multi.json"))
	assert.Nil(t, err)
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation")
	}
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing")
	}

	appPort := []string{"853/udp", "8000", "9000", "5432/tcp"}
	assert.EqualValues(t, &appPort, p.Config.AppPort)
}

// TestParseDevcontainerAppPortString parses a devcontainer.json with
// an appPort that consists of a single string and checks that the
// unmarshalled values match as expected
func TestParseDevcontainerAppPortString(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p, err := NewDevcontainerParser(filepath.Join("testdata", "parse", "devcontainer", "appport-single-string.json"))
	assert.Nil(t, err)
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation")
	}
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing")
	}

	appPort := []string{"6000"}
	assert.EqualValues(t, &appPort, p.Config.AppPort)
}

// TestParseDevcontainerForwardPorts parses a devcontainer.json that
// declares forwardPorts and validates that defaults port attributes
// are generated and applied
func TestParseDevcontainerForwardPorts(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p, err := NewDevcontainerParser(filepath.Join("testdata", "parse", "devcontainer", "forward-ports.json"))
	assert.Nil(t, err)
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

// TestParserDevcontainerFeatures parses a devcontainer.json that
// references a devcontainer Feature.
func TestParserDevcontainerFeatures(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p, err := NewDevcontainerParser(filepath.Join("testdata", "parse", "devcontainer", "features.json"))
	assert.Nil(t, err)
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation:", err)
	}
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing:", err)
	}

	assert.NotEmpty(t, p.Config.Features)
	for _, key := range []string{"features/shorthand", "features/with-options"} {
		assert.Contains(t, p.Config.Features, key)
	}

	// According to the spec, string values in shorthand feature
	// declarations should map to an option named "version":
	// https://containers.dev/implementors/features/#:~:text=This%20string%20is%20mapped%20to%20an%20option%20called%20version%2E
	assert.EqualValues(t, "1.0", *p.Config.Features["features/shorthand"]["version"].String)

	assert.EqualValues(t, "hello", *p.Config.Features["features/with-options"]["string-opt"].String)
	assert.True(t, *p.Config.Features["features/with-options"]["bool-opt"].Bool)
}

// TestParseDevcontainerLifecycle parses a devcontainer.json that
// declares lifecycle commands.
func TestParseDevcontainerLifecycle(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p, err := NewDevcontainerParser(filepath.Join("testdata", "parse", "devcontainer", "lifecycle.json"))
	assert.Nil(t, err)
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation:", err)
	}
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing:", err)
	}

	assert.Empty(t, p.Config.InitializeCommand.StringArray)
	assert.Empty(t, p.Config.OnCreateCommand.StringArray)
	assert.Empty(t, p.Config.UpdateContentCommand.StringArray)
	assert.Empty(t, p.Config.PostCreateCommand.StringArray)

	assert.Empty(t, p.Config.InitializeCommand.ParallelCommands)
	assert.Empty(t, p.Config.OnCreateCommand.ParallelCommands)
	assert.Empty(t, p.Config.UpdateContentCommand.ParallelCommands)
	assert.Empty(t, p.Config.PostCreateCommand.ParallelCommands)

	assert.EqualValues(t, "test", *p.Config.InitializeCommand.String)
	assert.EqualValues(t, "test", *p.Config.OnCreateCommand.String)
	assert.EqualValues(t, "test", *p.Config.UpdateContentCommand.String)
	assert.EqualValues(t, "test", *p.Config.PostCreateCommand.String)

	assert.NotEmpty(t, p.Config.PostStartCommand.StringArray)
	assert.Empty(t, p.Config.PostStartCommand.ParallelCommands)
	assert.EqualValues(t, "test", p.Config.PostStartCommand.StringArray[0])

	assert.Empty(t, p.Config.PostAttachCommand.String)
	assert.Empty(t, p.Config.PostAttachCommand.StringArray)
	assert.NotNil(t, p.Config.PostAttachCommand.ParallelCommands)

	assert.EqualValues(t, "test", *(*p.Config.PostAttachCommand.ParallelCommands)["cmd1"].String)
	assert.EqualValues(t, "test", (*p.Config.PostAttachCommand.ParallelCommands)["cmd2"].StringArray[0])
}

// TestParseDevcontainerMountStringList parses a devcontainer.json
// that declares mounts as a list of strings
func TestParseDevcontainerMountStringList(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p, err := NewDevcontainerParser(filepath.Join("testdata", "parse", "devcontainer", "mounts-string-list.json"))
	assert.Nil(t, err)
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation:", err)
	}
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing")
	}

	assert.EqualValues(t, "bind", p.Config.Mounts[0].Type)
	assert.EqualValues(t, "/vanilla-bind", p.Config.Mounts[0].Source)
	assert.EqualValues(t, "/vanilla-bind", p.Config.Mounts[0].Target)
	assert.False(t, p.Config.Mounts[0].ReadOnly)
	assert.Empty(t, p.Config.Mounts[0].Consistency)

	assert.EqualValues(t, "bind", p.Config.Mounts[1].Type)
	assert.EqualValues(t, "/readwrite-bind", p.Config.Mounts[1].Source)
	assert.EqualValues(t, "/readwrite-bind", p.Config.Mounts[1].Target)
	assert.False(t, p.Config.Mounts[1].ReadOnly)
	assert.Empty(t, p.Config.Mounts[1].Consistency)

	assert.EqualValues(t, "bind", p.Config.Mounts[2].Type)
	assert.EqualValues(t, "/readonly-bind-implicit", p.Config.Mounts[2].Source)
	assert.EqualValues(t, "/readonly-bind-implicit", p.Config.Mounts[2].Target)
	assert.True(t, p.Config.Mounts[2].ReadOnly)
	assert.Empty(t, p.Config.Mounts[2].Consistency)

	assert.EqualValues(t, "bind", p.Config.Mounts[3].Type)
	assert.EqualValues(t, "/readonly-bind-explicit", p.Config.Mounts[3].Source)
	assert.EqualValues(t, "/readonly-bind-explicit", p.Config.Mounts[3].Target)
	assert.True(t, p.Config.Mounts[3].ReadOnly)
	assert.Empty(t, p.Config.Mounts[3].Consistency)

	assert.EqualValues(t, "bind", p.Config.Mounts[4].Type)
	assert.EqualValues(t, "/tmp", p.Config.Mounts[4].Source)
	assert.EqualValues(t, "/tmp", p.Config.Mounts[4].Target)
	assert.False(t, p.Config.Mounts[4].ReadOnly)
	assert.EqualValues(t, "cached", p.Config.Mounts[4].Consistency)

	assert.EqualValues(t, "volume", p.Config.Mounts[5].Type)
	assert.EqualValues(t, "named-vol", p.Config.Mounts[5].Source)
	assert.EqualValues(t, "/named-vol", p.Config.Mounts[5].Target)
	assert.False(t, p.Config.Mounts[5].ReadOnly)
	assert.Empty(t, p.Config.Mounts[5].Consistency)
}

// TestParserDevcontainerPortsAttributes parses a devcontainer.json
// that declares forwardPorts *AND* portsAttributes and validates that
// explicit port attributes are able to override default values
func TestParserDevcontainerPortsAttributes(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	p, err := NewDevcontainerParser(filepath.Join("testdata", "parse", "devcontainer", "ports-attributes.json"))
	assert.Nil(t, err)
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

// TestParseDevcontainerVarExpansion exercises writ's variable
// expansion.
func TestParseDevcontainerVarExpansion(t *testing.T) {
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

	p, err := NewDevcontainerParser(filepath.Join("testdata", "parse", "devcontainer", "variable-expansion.json"))
	assert.Nil(t, err)
	if err := p.Validate(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed validation")
	}
	if err := p.Parse(); err != nil {
		t.Fatal("devcontainer.json expected to be valid failed parsing")
	}
	p.ProcessSubstitutions()

	containerEnv := EnvVarMap{
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
	assert.Equal(t, *p.Config.DockerFile, "devcontainer/Containerfile", "fields not matching")
	assert.Equal(t, p.Config.ContainerEnv, containerEnv, "fields not matching")

	for _, mount := range p.Config.Mounts {
		assert.Equal(t, localEnvVars["BRIG_TEST_MOUNT_SOURCE"], mount.Source)
		assert.Equal(t, localEnvVars["BRIG_TEST_MOUNT_TARGET"], mount.Target)
	}
}

// TestValidateDevcontainer attempts validation of known valid and
// invalid samples of devcontainer.json files.
func TestValidateDevcontainer(t *testing.T) {
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
				p, err := NewDevcontainerParser(path)
				assert.Nil(t, err)
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
				p, err := NewDevcontainerParser(path)
				assert.Nil(t, err)
				if err = p.Validate(); err == nil {
					t.Fatal("known-invalid sample passed validation: ", path)
				}
			}
		})
	}
}
