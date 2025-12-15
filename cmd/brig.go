package main

import "github.com/nlsantos/brig/internal/brig"

const AppName string = "brig"
const AppVersion string = "0.0.2-alpha"

func main() {
	brig.NewCommand(AppName, AppVersion)
}
