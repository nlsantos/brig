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
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/nlsantos/brig/internal/trill"
	"github.com/nlsantos/brig/writ"
	"golang.org/x/sync/errgroup"
)

// lifecycleHandler monitors the trill client's lifecycle channel and
// runs the appropriate hooks.
func (cmd *Command) lifecycleHandler(ctx context.Context, eg *errgroup.Group, p *writ.DevcontainerParser) (err error) {
	defer func() {
		cmd.trillClient.DevcontainerLifecycleResp <- err == nil
		close(cmd.trillClient.DevcontainerLifecycleResp)
	}()

	for event := range cmd.trillClient.DevcontainerLifecycleChan {
		switch event {
		case trill.LifecycleFeatureInstall:
			slog.Debug("lifecycle", "event", "feature:install")
			installDAG, err := cmd.BuildFeaturesInstallationGraph(&p.Config.OverrideFeatureInstallOrder)
			if err != nil {
				return err
			}
			roots := installDAG.GetRoots()
			for len(roots) > 0 {
				for raw := range maps.Values(roots) {
					featureParser, ok := raw.(*writ.DevcontainerFeatureParser)
					if !ok {
						return fmt.Errorf("value for vertex is of unexpected type")
					}

					featureInstallScript := filepath.Join(filepath.Dir(featureParser.Filepath), "install.sh")
					featureOptions := &writ.EnvVarMap{}
					for optName, opt := range featureParser.Config.Options {
						reAlphaNum := regexp.MustCompile(`[^\w_]`)
						reDigits := regexp.MustCompile(`^[\d_]+`)

						envKey := reAlphaNum.ReplaceAllLiteralString(optName, "_")
						envKey = reDigits.ReplaceAllLiteralString(envKey, "_")
						envKey = strings.ToUpper(envKey)

						switch opt.Type {
						case writ.FeatureOptionTypeBoolean:
							(*featureOptions)[envKey] = strconv.FormatBool(*opt.Value.Bool)

						case writ.FeatureOptionTypeString:
							(*featureOptions)[envKey] = *opt.Value.String
						}
					}

					if err = cmd.trillClient.ExecInDevcontainer(ctx, "root", featureOptions, false, featureInstallScript); err != nil {
						return err
					}
				}

				for id := range roots {
					if err := installDAG.DeleteVertex(id); err != nil {
						return err
					}
				}

				roots = installDAG.GetRoots()
			}

		case trill.LifecycleInitialize:
			slog.Debug("lifecycle", "event", "init")
			if p.Config.InitializeCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.InitializeCommand, p, true); err != nil {
					return err
				}
			}
			if *p.Config.WaitFor == writ.WaitForInitializeCommand {
				eg.Go(cmd.trillClient.AttachHostTerminalToDevcontainer)
			}

		case trill.LifecycleOnCreate:
			slog.Debug("lifecycle", "event", "onCreate")
			if p.Config.OnCreateCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.OnCreateCommand, p, false); err != nil {
					return err
				}
			}
			if *p.Config.WaitFor == writ.WaitForOnCreateCommand {
				eg.Go(cmd.trillClient.AttachHostTerminalToDevcontainer)
			}

		case trill.LifecyclePostAttach:
			slog.Debug("lifecycle", "event", "postAttach")
			if p.Config.PostAttachCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.PostAttachCommand, p, false); err != nil {
					return err
				}
			}

		case trill.LifecyclePostCreate:
			slog.Debug("lifecycle", "event", "postCreate")
			if p.Config.PostCreateCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.PostCreateCommand, p, false); err != nil {
					return err
				}
			}
			if *p.Config.WaitFor == writ.WaitForPostCreateCommand {
				eg.Go(cmd.trillClient.AttachHostTerminalToDevcontainer)
			}

		case trill.LifecyclePostStart:
			slog.Debug("lifecycle", "event", "postStart")
			if p.Config.PostStartCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.PostStartCommand, p, false); err != nil {
					return err
				}
			}
			if *p.Config.WaitFor == writ.WaitForPostStartCommand {
				eg.Go(cmd.trillClient.AttachHostTerminalToDevcontainer)
			}

		case trill.LifecycleUpdate:
			slog.Debug("lifecycle", "event", "update")
			if p.Config.UpdateContentCommand != nil {
				if err = cmd.runLifecycleCommand(ctx, p.Config.UpdateContentCommand, p, false); err != nil {
					return err
				}
			}
			if *p.Config.WaitFor == writ.WaitForUpdateContentCommand {
				eg.Go(cmd.trillClient.AttachHostTerminalToDevcontainer)
			}

		default:
			return fmt.Errorf("received unhandled lifecycle event: %v", event)
		}
		cmd.trillClient.DevcontainerLifecycleResp <- err == nil
	}

	slog.Debug("exiting lifecycle handler")
	return nil
}

// runLifecycleCommand determines which parameter of a given lifecycle
// command is active and runs it.
func (cmd *Command) runLifecycleCommand(ctx context.Context, lc *writ.LifecycleCommand, p *writ.DevcontainerParser, runOnHost bool) (err error) {
	switch {
	case lc.String != nil:
		if runOnHost {
			err = cmd.runLifecycleCommandOnHost(ctx, true, *lc.String)
		} else {
			err = cmd.runLifecycleCommandInContainer(ctx, p, true, *lc.String)
		}

	case len(lc.StringArray) > 0:
		if runOnHost {
			err = cmd.runLifecycleCommandOnHost(ctx, false, lc.StringArray...)
		} else {
			err = cmd.runLifecycleCommandInContainer(ctx, p, false, lc.StringArray...)
		}

	case lc.ParallelCommands != nil:
		var wg sync.WaitGroup
		errChan := make(chan error, len(*lc.ParallelCommands))
		for _, pcmd := range *lc.ParallelCommands {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errChan <- cmd.runLifecycleCommand(ctx, &writ.LifecycleCommand{CommandBase: pcmd}, p, runOnHost)
			}()
		}
		wg.Wait()
		close(errChan)
		for err = range errChan {
			if err != nil {
				return err
			}
		}
	}
	return err
}

// runLifecycleCommandInContainer executes a lifecycle command
// parameter inside the designated devcontainer (i.e., the lone
// container in non-Composer configurations, or the one named in the
// service field otherwise).
func (cmd *Command) runLifecycleCommandInContainer(ctx context.Context, p *writ.DevcontainerParser, runInShell bool, args ...string) error {
	return cmd.trillClient.ExecInDevcontainer(ctx, *p.Config.RemoteUser, &p.Config.RemoteEnv, runInShell, args...)
}

// runLifecycleCommandOnHost executes a lifecycle command parameter
// locally on the host.
func (cmd *Command) runLifecycleCommandOnHost(ctx context.Context, runInShell bool, args ...string) error {
	var execCmd *exec.Cmd

	if runInShell {
		shell := os.Getenv("SHELL")
		if len(shell) == 0 {
			shell = "/bin/sh"
		}
		slog.Info("running command via shell on host", "shell", shell, "args", args)
		args = append([]string{"-c"}, args...)
		execCmd = exec.CommandContext(ctx, shell, args...)
	} else {
		slog.Info("running command directly on host", "args", args)
		execCmd = exec.CommandContext(ctx, args[0], args[1:]...)
	}

	out, err := execCmd.CombinedOutput()
	slog.Info("command output", "cmd", execCmd.String(), "output", string(out), "error", err)
	return err
}
