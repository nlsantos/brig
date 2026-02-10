/*
   writ: a devcontainer.json parser
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

// Package writ houses a validating parser for devcontainer.json files
package writ

import (
	"encoding/json"
	"fmt"

	dockeropts "github.com/docker/cli/opts"
	dockermounts "github.com/docker/docker/volume/mounts"
)

// UnmarshalJSON for the AppPort type
func (a *AppPort) UnmarshalJSON(data []byte) error {
	// jscpd:ignore-start
	if len(data) < 1 {
		return nil
	}

	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var elements []string
	switch v := raw.(type) {
	case []any:
		for _, x := range v {
			switch y := x.(type) {
			case string:
				elements = append(elements, y)
			case float64:
				elements = append(elements, fmt.Sprintf("%.0f", y))
			}
		}
	case string:
		elements = append(elements, v)
	case float64:
		elements = append(elements, fmt.Sprintf("%.0f", v))
	default:
		fmt.Println(v)
		return fmt.Errorf("unknown type: %s", v)
	}
	*a = elements
	return nil
	// jscpd:ignore-end
}

// UnmarshalJSON for the CacheFrom type
func (c *CacheFrom) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch v := raw.(type) {
	case []any:
		var elements []string
		for _, x := range v {
			switch y := x.(type) {
			case string:
				elements = append(elements, y)
			default:
				return fmt.Errorf("unsupported type: %#v for value %#v", y, x)
			}
		}
		c.StringArray = elements

	case string:
		*c.String = v

	default:
		return fmt.Errorf("unsupported type: %#v for value %#v", v, raw)
	}

	return nil
}

// UnmarshalJSON for the CommandBase type
func (c *CommandBase) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch v := raw.(type) {
	case []any:
		var elements []string
		for _, x := range v {
			switch y := x.(type) {
			case string:
				elements = append(elements, y)
			default:
				return fmt.Errorf("unsupported type: %#v for value %#v", y, x)
			}
		}
		c.StringArray = elements

	case string:
		c.String = &v
	}

	return nil
}

// UnmarshalJSON for the DockerComposeFile type
func (d *DockerComposeFile) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var elements []string
	switch v := raw.(type) {
	case []any:
		for _, x := range v {
			switch y := x.(type) {
			case string:
				elements = append(elements, y)
			default:
				return fmt.Errorf("unsupported type: %#v for value %#v", y, x)
			}
		}

	case string:
		elements = append(elements, v)

	default:
		return fmt.Errorf("unsupported type: %#v for value %#v", v, raw)
	}

	*d = elements
	return nil
}

func (f *FeatureValues) UnmarshalJSON(data []byte) error {
	// Check if this is a shorthand feature declaration; according to
	// the spec, this should map to an option named "version":
	// https://containers.dev/implementors/features/#:~:text=This%20string%20is%20mapped%20to%20an%20option%20called%20version%2E
	if data[0] == '"' {
		if *f == nil {
			*f = make(FeatureValues)
		}

		versionOpt := FeatureValue{}
		if err := json.Unmarshal(data, &versionOpt); err != nil {
			return err
		}
		(*f)["version"] = versionOpt
		return nil
	}

	type longhandFeature FeatureValues
	return json.Unmarshal(data, (*longhandFeature)(f))
}

// UnmarshalJSON for the FeatureOptions type
func (f *FeatureValue) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &f.Bool); err == nil {
		return nil
	}

	if err := json.Unmarshal(data, &f.String); err == nil {
		f.Bool = nil
		return nil
	}

	return fmt.Errorf("feature option must be either a string or a boolean: %#v", data)
}

// UnmarshalJSON for the ForwardPort type
func (f *ForwardPorts) UnmarshalJSON(data []byte) error {
	// jscpd:ignore-start
	if len(data) < 1 {
		return nil
	}

	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var elements []string
	switch v := raw.(type) {
	case []any:
		for _, x := range v {
			switch y := x.(type) {
			case string:
				elements = append(elements, y)
			case float64:
				elements = append(elements, fmt.Sprintf("%.0f", y))
			}
		}
	case string:
		elements = append(elements, v)
	case float64:
		elements = append(elements, fmt.Sprintf("%.0f", v))
	default:
		fmt.Println(v)
		return fmt.Errorf("unknown type: %s", v)
	}
	*f = elements
	return nil
	// jscpd:ignore-end
}

// UnmarshalJSON for the LifecycleCommand type
func (l *LifecycleCommand) UnmarshalJSON(data []byte) error {
	err := l.CommandBase.UnmarshalJSON(data)
	if err != nil || l.String != nil || len(l.StringArray) > 0 {
		return err
	}

	var objMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &objMap); err != nil {
		return err
	}

	l.ParallelCommands = &map[string]CommandBase{}
	for key, raw := range objMap {
		var cmdBase CommandBase
		if err := json.Unmarshal(raw, &cmdBase); err != nil {
			return err
		}
		(*l.ParallelCommands)[key] = cmdBase
	}

	return nil
}

// UnmarshalJSON for the MobyMount type
func (m *MobyMount) UnmarshalJSON(data []byte) error {
	type mobyMount MobyMount
	if len(data) > 0 && data[0] == '{' {
		return json.Unmarshal(data, (*mobyMount)(m))
	}

	var mountString string
	if err := json.Unmarshal(data, &mountString); err != nil {
		return err
	}

	// Try parsing as the CSV type
	mountOpt := dockeropts.MountOpt{}
	if err := mountOpt.Set(mountString); err == nil {
		*m = (MobyMount)(mountOpt.Value()[0])
		return err
	}

	// Try parsing as the short version
	dockerParser := dockermounts.NewParser()
	mountPt, err := dockerParser.ParseMountRaw(mountString, "")
	if err == nil {
		specJSON, err := json.Marshal(mountPt.Spec)
		if err != nil {
			return err
		}
		return json.Unmarshal(specJSON, m)
	}

	return fmt.Errorf("unable to parse '%s' as a mount string", mountString)
}
