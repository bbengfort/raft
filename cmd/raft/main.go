package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/bbengfort/raft"
	pb "github.com/bbengfort/raft/api/v1beta1"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"

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
	app.Version = raft.Version()
	app.Usage = "implements the Raft consensus algorithm"
	app.Flags = []cli.Flag{}

	// Define commands available to application
	app.Commands = []*cli.Command{
		{
			Name:     "serve",
			Usage:    "run a raft replica server",
			Before:   initConfig,
			Action:   serve,
			Category: "server",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "config",
					Aliases: []string{"c"},
					Usage:   "configuration file for replica",
				},
				&cli.StringFlag{
					Name:    "name",
					Aliases: []string{"n"},
					Usage:   "unique name of the replica instance",
				},
				&cli.DurationFlag{
					Name:    "uptime",
					Aliases: []string{"u"},
					Usage:   "specify a duration for the server to run",
				},
				&cli.StringFlag{
					Name:    "outpath",
					Aliases: []string{"o"},
					Usage:   "write metrics to specified path",
				},
				&cli.Int64Flag{
					Name:    "seed",
					Aliases: []string{"s"},
					Usage:   "specify the random seed",
				},
				&cli.BoolFlag{
					Name:    "profile",
					Aliases: []string{"P"},
					Usage:   "enable go profiling",
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
				&cli.StringFlag{
					Name:    "config",
					Aliases: []string{"c"},
					Usage:   "configuration file for network",
				},
				&cli.StringFlag{
					Name:    "addr",
					Aliases: []string{"a"},
					Usage:   "name or address of replica to connect to",
				},
				&cli.StringFlag{
					Name:    "key",
					Aliases: []string{"k"},
					Usage:   "the name of the command to commit",
				},
				&cli.StringFlag{
					Name:    "value",
					Aliases: []string{"v"},
					Usage:   "the value of the command to commit",
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
				&cli.StringFlag{
					Name:    "config",
					Aliases: []string{"c"},
					Usage:   "configuration file for replica",
					Value:   "",
				},
				&cli.StringFlag{
					Name:    "addr",
					Aliases: []string{"a"},
					Usage:   "name or address of replica to connect to",
				},
				&cli.UintFlag{
					Name:    "requests",
					Aliases: []string{"r"},
					Usage:   "number of requests issued per client",
					Value:   1000,
				},
				&cli.IntFlag{
					Name:    "size",
					Aliases: []string{"s"},
					Usage:   "number of bytes per value",
					Value:   8192,
				},
				&cli.DurationFlag{
					Name:    "delay",
					Aliases: []string{"d"},
					Usage:   "wait specified time before starting benchmark",
				},
				&cli.IntFlag{
					Name:    "indent",
					Aliases: []string{"i"},
					Usage:   "indent the results by specified number of spaces",
				},
				&cli.BoolFlag{
					Name:    "blast",
					Aliases: []string{"b"},
					Usage:   "send all requests per client at once",
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
			return cli.Exit(err, 1)
		}

		if err = json.NewDecoder(f).Decode(&config); err != nil {
			return cli.Exit(err, 1)
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
		return cli.Exit(err.Error(), 1)
	}

	// TODO: uptime is only for benchmarking, remove when stable
	if uptime := c.Duration("uptime"); uptime > 0 {
		time.AfterFunc(uptime, func() {
			if path := c.String("outpath"); path != "" {
				extra := make(map[string]interface{})
				extra["replica"] = replica.Name
				extra["version"] = raft.Version()

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
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

//===========================================================================
// Client Commands
//===========================================================================

func commit(c *cli.Context) (err error) {
	if client, err = raft.NewClient(c.String("addr"), config); err != nil {
		return cli.Exit(err.Error(), 1)
	}

	var entry *pb.LogEntry
	if entry, err = client.Commit(c.String("key"), []byte(c.String("value"))); err != nil {
		return cli.Exit(err.Error(), 1)
	}

	fmt.Println(entry)

	return nil
}

func bench(c *cli.Context) error {
	if delay := c.Duration("delay"); delay > 0 {
		time.Sleep(delay)
	}

	if c.Bool("blast") && c.String("addr") == "" {
		return cli.Exit("blast requires the address of the leader specified", 1)
	}

	benchmark, err := raft.NewBenchmark(
		config, c.String("addr"), c.Bool("blast"), c.Uint("requests"), c.Uint("size"), c.Uint("workers"),
	)

	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	// Print the results
	results, err := benchmark.JSON(c.Int("indent"))
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	fmt.Println(string(results))
	return nil
}
