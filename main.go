/*
Copyright © 2024 defektive
*/
package main

import (
	"github.com/analog-substance/radon/pkg/cmd"
	"github.com/analog-substance/util/cli/docs"
	"github.com/analog-substance/util/cli/glamour_help"
	"github.com/analog-substance/util/cli/updater/cobra_updater"
	ver "github.com/analog-substance/util/cli/version"
)

var version = "v0.0.0"
var commit = "replace"

func main() {

	cmd.RootCmd.Version = ver.GetVersionInfo(version, commit)
	cobra_updater.AddToRootCmd(cmd.RootCmd)
	//completion.AddToRootCmd(cmd.RootCmd)
	cmd.RootCmd.AddCommand(docs.CobraDocsCmd)
	glamour_help.AddToRootCmd(cmd.RootCmd)

	cmd.RootCmd.Version = ver.GetVersionInfo(version, commit)
	cmd.Execute()
}
