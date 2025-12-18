//go:build !windows

package brig

import (
	"fmt"
	"log/slog"
	"os"

	"mvdan.cc/sh/v3/shell"
)

// Attempt to determine a viable socket address for communicating with
// Podman/Docker.
//
// If socketAddr is non-empty, this function just returns it
// immediately. Otherwise, it attempts to look for the DOCKER_HOST
// environment variable; failing that, it builds a path that will
// usually work for a system with Podman installed.
func getSocketAddr(socketAddr string) string {
	if len(socketAddr) > 0 {
		slog.Debug("received a non-empty socket address", "socket", socketAddr)
		return socketAddr
	}

	// Having Docker installed usually causes this to be set; the
	// podman-docker package (in its various guises across distros)
	// will also likely set this
	if envSocketAddr, ok := os.LookupEnv("DOCKER_HOST"); ok {
		slog.Debug("using socket nominated by DOCKER_HOST", "socket", envSocketAddr)
		return envSocketAddr
	}

	uid := os.Getuid()
	possibleSocketPaths := []string{
		"${XDG_RUNTIME_DIR}/docker.sock", // I'm pretty sure only podman-docker would cause this file to exist for a user
		"${XDG_RUNTIME_DIR}/podman/podman.sock",
		fmt.Sprintf("/run/user/%d/docker.sock", uid),
		fmt.Sprintf("/run/user/%d/podman/podman.sock", uid), // This also covers Podman + macOS, apparently?
		"/var/run/docker.sock",                              // Docker + GNU/Linux
		"/private/var/run/docker.sock",                      // Docker + macOS
	}

	for _, possibleSocketPath := range possibleSocketPaths {
		if socketPath, err := shell.Expand(possibleSocketPath, nil); err == nil {
			if _, err := os.Stat(socketPath); err == nil {
				slog.Debug("using possible socket found in filesystem", "socket", socketPath)
				// The protocol isn't strictly necessary; it seems the
				// Moby package automatically adds it as needed. Still...
				return fmt.Sprintf("unix://%s", socketPath)
			}
		}
	}

	slog.Error("unable to find a suitable socket address/path to target")
	return ""
}
