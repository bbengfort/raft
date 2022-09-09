package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/bbengfort/raft"
	"github.com/bbengfort/raft/pb"
	"github.com/joho/godotenv"
	"github.com/urfave/cli"

	_ "net/http/pprof"
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
				cli.BoolFlag{
					Name:  "P, profile",
					Usage: "enable go profiling",
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
					Name:  "a, addr",
					Usage: "name or address of replica to connect to",
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
				cli.StringFlag{
					Name:  "a, addr",
					Usage: "name or address of replica to connect to",
					Value: "",
				},
				cli.UintFlag{
					Name:  "r, requests",
					Usage: "number of requests issued per client",
					Value: 1000,
				},
				cli.IntFlag{
					Name:  "s, size",
					Usage: "number of bytes per value",
					Value: 32,
				},
				cli.DurationFlag{
					Name:  "d, delay",
					Usage: "wait specified time before starting benchmark",
				},
				cli.IntFlag{
					Name:  "i, indent",
					Usage: "indent the results by specified number of spaces",
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
		var f *os.File
		if f, err = os.Open(c.String("config")); err != nil {
			return cli.NewExitError(err, 1)
		}

		if err = json.NewDecoder(f).Decode(&config); err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	return nil
}

//===========================================================================
// Server Commands
//===========================================================================

func serve(c *cli.Context) (err error) {
	if c.Bool("profile") {
		go func() {
			fmt.Println("==== PROFILING ENABLED ==========")
			runtime.SetBlockProfileRate(5000)
			err := http.ListenAndServe("0.0.0.0:6060", nil)
			panic(err)
		}()
	}

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
	if client, err = raft.NewClient(c.String("addr"), config); err != nil {
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

	if c.Bool("blast") && c.String("addr") == "" {
		return cli.NewExitError("blast requires the address of the leader specified", 1)
	}

	benchmark, err := raft.NewBenchmark(
		config, c.String("addr"), c.Bool("blast"), c.Uint("requests"), c.Uint("size"), c.Uint("workers"),
	)

	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	// Print the results
	results, err := benchmark.JSON(c.Int("indent"))
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	fmt.Println(string(results))
	return nil
}
