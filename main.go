/*
Copyright © 2024 defektive
*/
package main

import (
	"github.com/analog-substance/radon/pkg/cmd"
	ver "github.com/analog-substance/util/cli/version"
)

var version = "v0.0.0"
var commit = "replace"

func main() {
	cmd.RootCmd.Version = ver.GetVersionInfo(version, commit)
	cmd.Execute()
}
