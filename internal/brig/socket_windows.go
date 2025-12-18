//go:build windows

package brig

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Attempt to determine a viable socket address for communicating with
// Podman/Docker.
//
// If socketAddr is non-empty, this function just returns it
// immediately. Otherwise, it attempts to check if certain named pipes exist; if
// one of them does, returns the string.  If no viable named pipes are found,
// returns an empty string.
func getSocketAddr(socketAddr string) string {
	if len(socketAddr) > 0 {
		slog.Debug("received a non-empty socket address", "socket", socketAddr)
		return socketAddr
	}

	const pipeProto string = "npipe://"
	retval := ""
	possibleNamedPipes := []string{
		`\\.\pipe\podman-machine-default`,
		`\\.\pipe\docker_engine`,
	}

	for _, possibleNamedPipe := range possibleNamedPipes {
		if _, err := os.Stat(possibleNamedPipe); err == nil {
			slog.Debug("using possible named pipe found in filesystem", "named-pipe", possibleNamedPipe)
			retval = possibleNamedPipe
		}
	}

	if len(retval) == 0 {
		slog.Error("unable to find a suitable named pipe to target")
	} else if !strings.HasPrefix(retval, pipeProto) {
		// The protocol seems to be mandatory for named pipes
		return fmt.Sprintf("%s%s", pipeProto, filepath.ToSlash(retval))
	}

	return retval
}
