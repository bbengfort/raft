package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/bbengfort/raft"
	"github.com/bbengfort/raft/pb"
	"github.com/joho/godotenv"
	"github.com/urfave/cli"
)

// Command variables
var (
	config  *raft.Config
	replica *raft.Replica
	client  *raft.Client
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
		{
			Name:     "commit",
			Usage:    "commit an entry to the distributed log",
			Before:   initConfig,
			Action:   commit,
			Category: "client",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "c, config",
					Usage: "configuration file for replica",
					Value: "",
				},
				cli.StringFlag{
					Name:  "k, key",
					Usage: "the name of the command to commit",
				},
				cli.StringFlag{
					Name:  "v, value",
					Usage: "the value of the command to commit",
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

//===========================================================================
// Client Commands
//===========================================================================

func commit(c *cli.Context) (err error) {
	if client, err = raft.NewClient(config); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	var entry *pb.LogEntry
	if entry, err = client.Commit(c.String("key"), []byte(c.String("value"))); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	fmt.Println(entry)

	return nil
}
