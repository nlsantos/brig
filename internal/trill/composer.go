/*
   trill: a lightweight wrapper for Podman/Docker REST API calls
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

// Package trill houses a thin wrapper for communicating with podman
// and Docker via their REST API.
package trill

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	compose "github.com/compose-spec/compose-go/cli"
	composetypes "github.com/compose-spec/compose-go/types"
	"github.com/davecgh/go-spew/spew"
	"github.com/heimdalr/dag"

	dockerspecs "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/moby/moby/api/types/container"
	mobyclient "github.com/moby/moby/client"
	"github.com/nlsantos/brig/writ"
)

type ServiceWalker struct{}

func (w ServiceWalker) Visit(v dag.Vertexer) {
	vertexID, _ := v.Vertex()
	spew.Dump(vertexID)
}

func (c *Client) DeployComposerProject(p *writ.Parser, projName string, imageTagPrefix string, suppressOutput bool) error {
	projOptions, err := compose.NewProjectOptions(
		[]string(*p.Config.DockerComposeFile),
		compose.WithConsistency(true),
		compose.WithContext(context.Background()),
		compose.WithInterpolation(true),
		compose.WithName(projName), // Maybe overriding the name can be a flag?
		compose.WithNormalization(true),
		compose.WithResolvedPaths(true),
		compose.WithWorkingDirectory(*p.Config.Context),
	)
	if err != nil {
		return err
	}

	c.composerProject, err = compose.ProjectFromOptions(projOptions)
	if err != nil {
		return err
	}

	c.servicesDAG = dag.NewDAG()
	// First, add the vertices, to make sure the edges will have a
	// valid reference...
	for _, service := range c.composerProject.AllServices() {
		if err := c.servicesDAG.AddVertexByID(service.Name, &service); err != nil {
			return err
		}
	}
	// ... then the edges
	for _, service := range c.composerProject.AllServices() {
		for _, dependency := range service.GetDependencies() {
			if err := c.servicesDAG.AddEdge(dependency, service.Name); err != nil {
				return err
			}
		}
	}

	// Check that p.Config.Service is named as a container in the
	// Composer project, otherwise we won't know which one to attach
	// to.
	if _, err := c.servicesDAG.GetVertex(*p.Config.Service); err != nil {
		slog.Debug("service container in devcontainer.json not named in composer YAML", "service", *p.Config.Service, "vertices", maps.Keys(c.servicesDAG.GetVertices()))
		return fmt.Errorf("service container in devcontainer.json not named in composer YAML: %s", *p.Config.Service)
	}

	if err := c.createComposerNetworks(c.composerProject.Networks); err != nil {
		return err
	}

	if err := c.createComposerVolumes(c.composerProject.Volumes); err != nil {
		return err
	}

	spinUpDAG, err := c.servicesDAG.Copy()
	if err != nil {
		return nil
	}
	if err := c.createComposerServices(p, spinUpDAG, imageTagPrefix, suppressOutput); err != nil {
		return err
	}

	return nil
}

func (c *Client) TeardownComposerProject() error {
	slog.Debug("tearing down resources related to the Composer project")
	ctx := context.Background()
	for _, networkCfg := range c.composerProject.Networks {
		slog.Debug("removing generated network", "network", networkCfg.Name)
		if _, err := c.mobyClient.NetworkRemove(ctx, networkCfg.Name, mobyclient.NetworkRemoveOptions{}); err != nil {
			return err
		}
	}

	teardownDAG, err := c.servicesDAG.Copy()
	if err != nil {
		return err
	}
	if err := c.teardownComposerServices(teardownDAG); err != nil {
		return err
	}
	return nil
}

func (c *Client) createComposerNetworks(networks map[string]composetypes.NetworkConfig) error {
	for _, networkCfg := range networks {
		// TODO: Look up how this is supposed to be handled in the Compose spec
		if networkCfg.External.External {
			slog.Debug("network defines an unsupported External configuration", "network", networkCfg.Name)
			continue
		}

		networkCreateOpts, err := convertNetworkConfig(networkCfg)
		if err != nil {
			return err
		}
		res, err := c.mobyClient.NetworkCreate(context.Background(), networkCfg.Name, *networkCreateOpts)
		if err != nil {
			return err
		}
		for _, warning := range res.Warning {
			slog.Warn(warning)
		}
	}
	return nil
}

func (c *Client) buildServiceContainerConfig(p *writ.Parser, serviceCfg *composetypes.ServiceConfig) *container.Config {
	isServiceContainer := *p.Config.Service == serviceCfg.Name

	// We mostly want the Env field and some defaults set...
	containerCfg := c.buildContainerConfig(p, serviceCfg.Image)
	// ... we overwrite where needed
	containerCfg.Hostname = serviceCfg.Hostname
	containerCfg.Domainname = serviceCfg.DomainName
	containerCfg.Tty = serviceCfg.Tty
	containerCfg.OpenStdin = serviceCfg.StdinOpen
	containerCfg.Cmd = serviceCfg.Command
	containerCfg.Entrypoint = serviceCfg.Entrypoint
	containerCfg.Labels = serviceCfg.Labels
	containerCfg.StopSignal = serviceCfg.StopSignal

	if serviceCfg.Attach != nil {
		containerCfg.AttachStdin = *serviceCfg.Attach
		containerCfg.AttachStdout = *serviceCfg.Attach
		containerCfg.AttachStderr = *serviceCfg.Attach
	}

	if serviceCfg.HealthCheck != nil && !serviceCfg.HealthCheck.Disable {
		containerCfg.Healthcheck = &dockerspecs.HealthcheckConfig{
			Test:          serviceCfg.HealthCheck.Test,
			Interval:      time.Duration(*serviceCfg.HealthCheck.Interval),
			Timeout:       time.Duration(*serviceCfg.HealthCheck.Timeout),
			StartPeriod:   time.Duration(*serviceCfg.HealthCheck.StartPeriod),
			StartInterval: time.Duration(*serviceCfg.HealthCheck.StartInterval),
			Retries:       int(*serviceCfg.HealthCheck.Retries),
		}
	}

	for key, val := range serviceCfg.Environment {
		if val != nil {
			containerCfg.Env = append(containerCfg.Env, fmt.Sprintf("%s=%s", key, *val))
		} else if localEnv, ok := os.LookupEnv(key); ok {
			containerCfg.Env = append(containerCfg.Env, fmt.Sprintf("%s=%s", key, localEnv))
		}
	}

	if isServiceContainer {
		if p.Config.ContainerUser != nil {
			containerCfg.User = *p.Config.ContainerUser
		}
	} else {
		containerCfg.User = serviceCfg.User
		containerCfg.WorkingDir = serviceCfg.WorkingDir
	}

	return containerCfg
}

func (c *Client) buildServiceHostConfig(p *writ.Parser, serviceCfg *composetypes.ServiceConfig) *container.HostConfig {
	isServiceContainer := *p.Config.Service == serviceCfg.Name
	hostCfg := container.HostConfig{
		AutoRemove:     false, // This is handled when the project is torn down
		CapAdd:         serviceCfg.CapAdd,
		CapDrop:        serviceCfg.CapDrop,
		DNSOptions:     serviceCfg.DNSOpts,
		GroupAdd:       serviceCfg.GroupAdd,
		IpcMode:        container.IpcMode(serviceCfg.Ipc),
		Cgroup:         container.CgroupSpec(serviceCfg.Cgroup),
		Links:          serviceCfg.Links,
		OomScoreAdj:    int(serviceCfg.OomScoreAdj),
		PidMode:        container.PidMode(serviceCfg.Pid),
		Privileged:     serviceCfg.Privileged,
		ReadonlyRootfs: serviceCfg.ReadOnly,
		SecurityOpt:    serviceCfg.SecurityOpt,
		UTSMode:        container.UTSMode(serviceCfg.Uts),
		UsernsMode:     container.UsernsMode(serviceCfg.UserNSMode),
		ShmSize:        int64(serviceCfg.ShmSize),
		Sysctls:        serviceCfg.Sysctls,
		Runtime:        serviceCfg.Runtime,
		Isolation:      container.Isolation(serviceCfg.Isolation),
		Init:           serviceCfg.Init,
	}

	for _, dns := range serviceCfg.DNS {
		hostCfg.DNS = append(hostCfg.DNS, netip.MustParseAddr(dns))
	}

	for host, addr := range serviceCfg.ExtraHosts {
		hostCfg.ExtraHosts = append(hostCfg.ExtraHosts, fmt.Sprintf("%s:%s", host, addr))
	}

	for _, mount := range serviceCfg.Tmpfs {
		hostCfg.Tmpfs[mount] = ""
	}

	if isServiceContainer && c.MakeMeRoot {
		hostCfg.UsernsMode = "keep-id:uid=0,gid=0"
	}

	return &hostCfg
}

func (c *Client) buildServiceBuildOpts(buildCfg *composetypes.BuildConfig, suppressOutput bool) (buildOpts *mobyclient.ImageBuildOptions, err error) {
	if buildCfg == nil {
		return nil, nil
	}

	if len(buildCfg.DockerfileInline) > 0 {
		containerfilePath, err := c.synthesizeInlineContainerfile(buildCfg.Context, &buildCfg.DockerfileInline)
		if err != nil {
			slog.Error("encountered an error while attempting to synthesize a Containerfile from an inlined one", "error", err)
			return nil, err
		}
		buildCfg.Dockerfile = containerfilePath
	}

	buildOpts = &mobyclient.ImageBuildOptions{
		Tags:           buildCfg.Tags,
		SuppressOutput: suppressOutput,
		NoCache:        buildCfg.NoCache,
		PullParent:     buildCfg.Pull,
		Isolation:      container.Isolation(buildCfg.Isolation),
		NetworkMode:    buildCfg.Network, // This might not be equivalent
		Dockerfile:     buildCfg.Dockerfile,
		BuildArgs:      buildCfg.Args,
		Labels:         buildCfg.Labels,
		CacheFrom:      buildCfg.CacheFrom,
		Target:         buildCfg.Target,
	}

	for name, uliimit := range buildCfg.Ulimits {
		buildOpts.Ulimits = append(buildOpts.Ulimits, &container.Ulimit{
			Name: name,
			Hard: int64(uliimit.Hard),
			Soft: int64(uliimit.Soft),
		})
	}

	return buildOpts, err
}

func (c *Client) waitForServiceDependencies(dependsOn *composetypes.DependsOnConfig) {
	//var wg sync.WaitGroup

	for containerName, dependency := range *dependsOn {
		spew.Dump(containerName)
		spew.Dump(dependency)
	}
}

func (c *Client) createComposerService(p *writ.Parser, serviceCfg *composetypes.ServiceConfig, imageTagPrefix string, suppressOutput bool) (err error) {
	containerName := fmt.Sprintf("%s--%s", c.composerProject.Name, serviceCfg.Name)
	imageTag := fmt.Sprintf("%s%s", imageTagPrefix, containerName)
	slog.Debug("converting service config to Moby equivalents", "name", containerName)

	c.waitForServiceDependencies(&serviceCfg.DependsOn)

	containerCfg := c.buildServiceContainerConfig(p, serviceCfg)
	hostCfg := c.buildServiceHostConfig(p, serviceCfg)
	if serviceCfg.Build != nil {
		buildOpts, err := c.buildServiceBuildOpts(serviceCfg.Build, suppressOutput)
		if err != nil {
			return err
		}
		buildOpts.Tags = append(buildOpts.Tags, imageTag)
		fmt.Println(serviceCfg.Build.Context)
		time.Sleep(2 * time.Second)
		if err = c.BuildContainerImage(serviceCfg.Build.Context, serviceCfg.Build.Dockerfile, imageTag, buildOpts, suppressOutput); err != nil {
			return err
		}
		containerCfg.Image = imageTag
	} else if len(serviceCfg.Image) > 0 {
		if err = c.PullContainerImage(serviceCfg.Image, suppressOutput); err != nil {
			return err
		}
		containerCfg.Image = serviceCfg.Image
	}

	fmt.Println(containerName)
	slog.Debug("using container config", "config", containerCfg)
	slog.Debug("using host config", "config", hostCfg)
	slog.Debug("creating Composer service container", "name", containerName)
	ctx := context.Background()
	createResp, err := c.mobyClient.ContainerCreate(ctx, mobyclient.ContainerCreateOptions{
		Config:     containerCfg,
		HostConfig: hostCfg,
		Name:       containerName,
	})
	if err != nil {
		slog.Error("encountered an error creating a container", "error", err)
		return err
	}
	slog.Debug("Composer container created successfully", "id", createResp.ID)

	slog.Debug("attempting to start Composer container", "id", createResp.ID)
	if _, err = c.mobyClient.ContainerStart(ctx, createResp.ID, mobyclient.ContainerStartOptions{}); err != nil {
		slog.Error("encountered an error while trying to start Composer container", "id", createResp.ID)
		return err
	}
	slog.Debug("Composer container started successfully", "id", createResp.ID)
	return nil
}

func (c *Client) synthesizeInlineContainerfile(contextPath string, inlinedContainerfile *string) (containerfilePath string, err error) {
	containerfilePath = filepath.Join(contextPath, "Containerfile")
	cf, err := os.OpenFile(containerfilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			return
		}
		err = cf.Close()
	}()
	_, err = cf.WriteString(*inlinedContainerfile)
	return containerfilePath, err
}

func (c *Client) createComposerServices(p *writ.Parser, servicesDAG *dag.DAG, imageTagPrefix string, suppressOutput bool) error {
	roots := servicesDAG.GetRoots()
	for len(roots) > 0 {
		var wg sync.WaitGroup
		errChan := make(chan error, len(roots))

		for raw := range maps.Values(roots) {
			serviceCfg, ok := raw.(*composetypes.ServiceConfig)
			if !ok {
				return fmt.Errorf("value for vertex is of unexpected type")
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				errChan <- c.createComposerService(p, serviceCfg, imageTagPrefix, suppressOutput)
			}()
		}
		wg.Wait()
		close(errChan)

		for err := range errChan {
			if err != nil {
				return err
			}
		}

		for id := range roots {
			if err := servicesDAG.DeleteVertex(id); err != nil {
				return err
			}
		}

		roots = servicesDAG.GetRoots()
	}

	return nil
}

func (c *Client) teardownComposerServices(servicesDAG *dag.DAG) error {
	leaves := servicesDAG.GetLeaves()
	for len(leaves) > 0 {
		var wg sync.WaitGroup
		errChan := make(chan error, len(leaves))

		for raw := range maps.Values(leaves) {
			serviceCfg, ok := raw.(*composetypes.ServiceConfig)
			if !ok {
				return fmt.Errorf("value for vertex is of unexpected type")
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				containerName := fmt.Sprintf("%s--%s", c.composerProject.Name, serviceCfg.Name)
				slog.Info("stopping and removing Composer container", "container", containerName)
				if _, err := c.mobyClient.ContainerStop(context.Background(), containerName, mobyclient.ContainerStopOptions{}); err != nil {
					errChan <- err
					return
				}
				if _, err := c.mobyClient.ContainerRemove(context.Background(), containerName, mobyclient.ContainerRemoveOptions{}); err != nil {
					errChan <- err
				}
			}()
		}
		wg.Wait()
		close(errChan)

		for err := range errChan {
			if err != nil {
				return err
			}
		}

		for id := range leaves {
			if err := servicesDAG.DeleteVertex(id); err != nil {
				return err
			}
		}

		leaves = servicesDAG.GetLeaves()
	}

	return nil
}

func (c *Client) createComposerVolumes(volumes composetypes.Volumes) error {
	slog.Warn("COMPOSER VOLUMES IS UNIMPLEMENTED")
	for _, volumeCfg := range volumes {
		slog.Debug(fmt.Sprintf("%#v", volumeCfg))
	}
	return nil
}

func convertNetworkConfig(networkCfg composetypes.NetworkConfig) (*mobyclient.NetworkCreateOptions, error) {
	// TODO: Implement conversion
	if len(networkCfg.Ipam.Driver) > 0 || networkCfg.Ipam.Config != nil {
		slog.Error("network config conversion for IPAM config is not yet implemented", "ipamcfg", networkCfg.Ipam)
		return nil, fmt.Errorf("network config relies on unimplemented functionality")
	}

	defTrue := true
	nco := mobyclient.NetworkCreateOptions{
		Driver:     networkCfg.Driver,
		Scope:      "local",
		EnableIPv4: &defTrue,
		EnableIPv6: &networkCfg.EnableIPv6,
		Internal:   networkCfg.Internal,
		Attachable: networkCfg.Attachable,
		Ingress:    false,
		ConfigOnly: false,
	}
	return &nco, nil
}
