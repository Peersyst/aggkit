package main

import (
	"os"

	"github.com/agglayer/aggkit"
	"github.com/agglayer/aggkit/common"
	"github.com/agglayer/aggkit/config"
	"github.com/agglayer/aggkit/log"
	"github.com/urfave/cli/v2"
)

const appName = "aggkit"

const (
	// NETWORK_CONFIGFILE name to identify the network_custom (genesis) config-file
	NETWORK_CONFIGFILE = "custom_network" //nolint:stylecheck
)

var (
	configFileFlag = cli.StringSliceFlag{
		Name:     config.FlagCfg,
		Aliases:  []string{"c"},
		Usage:    "Configuration file(s)",
		Required: false,
	}
	componentsFlag = cli.StringSliceFlag{
		Name:     config.FlagComponents,
		Aliases:  []string{"co"},
		Usage:    "List of components to run",
		Required: false,
		Value:    cli.NewStringSlice(common.AGGORACLE, common.AGGSENDER, common.BRIDGE),
	}
	saveConfigFlag = cli.StringFlag{
		Name:     config.FlagSaveConfigPath,
		Aliases:  []string{"s"},
		Usage:    "Save final configuration into to the indicated path (name: aggkit-config.toml)",
		Required: false,
	}
	disableDefaultConfigVars = cli.BoolFlag{
		Name:     config.FlagDisableDefaultConfigVars,
		Aliases:  []string{"d"},
		Usage:    "Disable default configuration variables, all of them must be defined on config files",
		Required: false,
	}
	allowDeprecatedFields = cli.BoolFlag{
		Name:     config.FlagAllowDeprecatedFields,
		Usage:    "Allow that config-files contains deprecated fields",
		Required: false,
	}
)

func main() {
	app := cli.NewApp()
	app.Name = appName
	app.Version = aggkit.Version
	flags := []cli.Flag{
		&configFileFlag,
		&componentsFlag,
		&saveConfigFlag,
		&disableDefaultConfigVars,
		&allowDeprecatedFields,
	}
	app.Commands = []*cli.Command{
		{
			Name:    "version",
			Aliases: []string{},
			Usage:   "Application version and build",
			Action:  versionCmd,
		},
		{
			Name:    "run",
			Aliases: []string{},
			Usage:   "Run the aggkit client",
			Action:  start,
			Flags:   flags,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
