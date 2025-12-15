//go:build windows

package trill

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

// Hooks into terminal resize signals on Windows
func (c *Client) listenForTerminalResize() {
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		fd := int(os.Stdout.Fd())
		if !term.IsTerminal(fd) {
			fmt.Println("not a terminal")
			return
		}

		for range ticker.C {
			w, h, err := term.GetSize(fd)
			if err != nil {
				panic(err)
			}
			c.ResizeContainer(uint(h), uint(w))
		}
	}()
}
