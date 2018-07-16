package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/bbengfort/raft"
	"github.com/joho/godotenv"
	"github.com/urfave/cli"
)

// Command variables
var (
	config  *raft.Config
	replica *raft.Replica
)

func main() {

	// Load the .env file if it exists
	godotenv.Load()

	// Instantiate the command line application
	app := cli.NewApp()
	app.Name = "raft"
	app.Version = raft.PackageVersion
	app.Usage = "implements the Raft consensus algorithm"

	// Define commands available to application
	app.Commands = []cli.Command{
		{
			Name:     "serve",
			Usage:    "run a raft replica server",
			Before:   initConfig,
			Action:   serve,
			Category: "server",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "c, config",
					Usage: "configuration file for replica",
					Value: "",
				},
			},
		},
	}

	// Run the CLI program
	app.Run(os.Args)
}

//===========================================================================
// Initialization
//===========================================================================

func initConfig(c *cli.Context) (err error) {
	if c.String("config") != "" {
		data, err := ioutil.ReadFile(c.String("config"))
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}

		if err = json.Unmarshal(data, &config); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	}
	return nil
}

//===========================================================================
// Server Commands
//===========================================================================

func serve(c *cli.Context) (err error) {

	if replica, err = raft.New(config); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if err = replica.Listen(); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	return nil
}
