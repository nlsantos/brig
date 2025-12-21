//go:build !windows

package trill

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

// Hooks into terminal resize signals on *nix
func (c *Client) listenForTerminalResize() {
	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, syscall.SIGWINCH)

	go func() {
		for range resizeCh {
			fd := int(os.Stdin.Fd())
			if !term.IsTerminal(fd) {
				return
			}
			w, h, err := term.GetSize(fd)
			if err != nil {
				return
			}
			c.ResizeContainer(uint(h), uint(w)) // #nosec G115
		}
	}()
}
