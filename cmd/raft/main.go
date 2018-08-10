package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

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
				cli.StringFlag{
					Name:  "n, name",
					Usage: "unique name of the replica instance",
					Value: "",
				},
				cli.DurationFlag{
					Name:  "u, uptime",
					Usage: "specify a duration for the server to run",
					Value: 0,
				},
				cli.StringFlag{
					Name:  "o, outpath",
					Usage: "write metrics to specified path",
					Value: "",
				},
				cli.Int64Flag{
					Name:  "s, seed",
					Usage: "specify the random seed",
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
					Usage: "configuration file for network",
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
		{
			Name:     "bench",
			Usage:    "run a raft benchmark with concurrent network",
			Before:   initConfig,
			Action:   bench,
			Category: "client",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "c, config",
					Usage: "configuration file for replica",
					Value: "",
				},
				cli.IntFlag{
					Name:  "n, nclients",
					Usage: "number of concurrent clients to run",
					Value: 4,
				},
				cli.Uint64Flag{
					Name:  "r, requests",
					Usage: "number of requests issued per client",
					Value: 1000,
				},
				cli.StringFlag{
					Name:  "o, outpath",
					Usage: "write metrics to specified path",
					Value: "",
				},
				cli.DurationFlag{
					Name:  "d, delay",
					Usage: "wait specified time before starting benchmark",
					Value: 0,
				},
				cli.BoolFlag{
					Name:  "b, blast",
					Usage: "send all requests per client at once",
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

	if name := c.String("name"); name != "" {
		config.Name = name
	}

	if seed := c.Int64("seed"); seed > 0 {
		config.Seed = seed
	}

	if replica, err = raft.New(config); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	// TODO: uptime is only for benchmarking, remove when stable
	if uptime := c.Duration("uptime"); uptime > 0 {
		time.AfterFunc(uptime, func() {
			if path := c.String("outpath"); path != "" {
				extra := make(map[string]interface{})
				extra["replica"] = replica.Name
				extra["version"] = raft.PackageVersion

				quorum := make([]string, 0, len(config.Peers))
				for _, peer := range config.Peers {
					quorum = append(quorum, peer.Name)
				}
				extra["quorum"] = quorum

				if err = replica.Metrics.Dump(path, extra); err != nil {
					fmt.Println(err.Error())
					os.Exit(1)
				}
			}
			os.Exit(0)
		})
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

func bench(c *cli.Context) error {
	if delay := c.Duration("delay"); delay > 0 {
		time.Sleep(delay)
	}

	benchmark, err := raft.NewBenchmark(
		config, c.Int("nclients"), c.Uint64("requests"), c.Bool("blast"),
	)

	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if path := c.String("outpath"); path != "" {
		if err := benchmark.Dump(path); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
	}

	fmt.Println(benchmark)
	return nil
}
