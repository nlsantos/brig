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

// Package main houses the entrypoint for the brig CLI
package main

import (
	"os"

	"github.com/nlsantos/brig/internal/brig"
)

const AppName string = "brig"
const AppVersion string = "0.0.13-alpha"

func main() {
	os.Exit(int(brig.NewCommand(AppName, AppVersion)))
}
