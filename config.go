package raft

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/bbengfort/x/peers"
	"github.com/fatih/structs"
	"github.com/koding/multiconfig"
)

// Config uses the multiconfig loader and validators to store configuration
// values required to run Raft. Configuration can be stored as a JSON, TOML,
// or YAML file in the current working directory as raft.json, in the user's
// home directory as .raft.json or in /etc/raft.json (with the extension of
// the file format of choice). Configuration can also be added from the
// environment using environment variables prefixed with $RAFT_ and the all
// caps version of the configuration name.
type Config struct {
	Name     string       `required:"false" json:"name"`                       // unique name of the local replica, hostname by default
	Seed     int64        `required:"false" json:"seed"`                       // random seed to initialize random generator
	Tick     string       `default:"1s" validate:"duration" json:"tick"`       // clock tick rate for timing (parseable duration)
	Timeout  string       `default:"500ms" validate:"duration" json:"timeout"` // timeout to wait for responses (parseable duration)
	LogLevel int          `default:"3" validate:"uint" json:"log_level"`       // verbosity of logging, lower is more verbose
	Leader   string       `required:"false" json:"leader,omitempty"`           // designated initial leader, if any
	Peers    []peers.Peer `json:"peers"`                                       // definition of all hosts on the network

	// Experimental configuration
	// TODO: remove after benchmarks
	Uptime  string `required:"false" validate:"duration" json:"uptime"` // run for a time limit and then shutdown
	Metrics string `requred:"false" json:"metrics"`                     // location to write benchmarks to disk
}

// Load the configuration from default values, then from a configuration file,
// and finally from the environment. Validate the configuration when loaded.
func (c *Config) Load() error {
	loaders := []multiconfig.Loader{}

	// Read default values defined via tag fields "default"
	loaders = append(loaders, &multiconfig.TagLoader{})

	// Find the config path and hte appropriate file loader
	if path, err := c.GetPath(); err == nil {
		if strings.HasSuffix(path, "toml") {
			loaders = append(loaders, &multiconfig.TOMLLoader{Path: path})
		}

		if strings.HasSuffix(path, "json") {
			loaders = append(loaders, &multiconfig.JSONLoader{Path: path})
		}

		if strings.HasSuffix(path, "yml") || strings.HasSuffix(path, "yaml") {
			loaders = append(loaders, &multiconfig.YAMLLoader{Path: path})
		}

	}

	// Load the environment variable loader
	env := &multiconfig.EnvironmentLoader{Prefix: "RAFT", CamelCase: true}
	loaders = append(loaders, env)

	loader := multiconfig.MultiLoader(loaders...)
	if err := loader.Load(c); err != nil {
		return err
	}

	return c.Validate()
}

// Validate the loaded configuration using the multiconfig multi validator.
func (c *Config) Validate() error {
	validators := multiconfig.MultiValidator(
		&multiconfig.RequiredValidator{},
		&ComplexValidator{},
	)

	return validators.Validate(c)
}

// Update the configuration from another configuration struct
func (c *Config) Update(o *Config) error {
	if o == nil {
		return nil
	}

	conf := structs.New(c)

	// Then update the current config with values from the other config
	for _, field := range structs.Fields(o) {
		if !field.IsZero() {
			updateField := conf.Field(field.Name())
			updateField.Set(field.Value())
		}
	}

	return c.Validate()
}

// GetName returns the name of the local host defined by the configuration or
// using the hostname by default.
func (c *Config) GetName() (name string, err error) {
	if c.Name == "" {
		if name, err = os.Hostname(); err != nil {
			return "", errors.New("could not find  unique name of localhost")
		}
		return name, nil
	}

	return c.Name, nil
}

// GetPeer returns the local peer configuration or an error if no peer is
// found in the configuration. If the name is not set on the configuration,
// the hostname is used.
func (c *Config) GetPeer() (peers.Peer, error) {
	local, err := c.GetName()
	if err != nil {
		return peers.Peer{}, err
	}

	for _, peer := range c.Peers {
		if peer.Name == local {
			return peer, nil
		}
	}

	return peers.Peer{}, fmt.Errorf("could not find peer for '%s'", local)
}

// IsLeader returns true if the local replica is the leader.
func (c *Config) IsLeader() bool {
	if c.Leader != "" {
		name, _ := c.GetName()
		return name == c.Leader
	}
	return false
}

// GetRemotes returns all peer configurations for remote hosts on the network,
// e.g. by excluding the local peer configuration.
func (c *Config) GetRemotes() (remotes []peers.Peer, err error) {
	var local string
	if local, err = c.GetName(); err != nil {
		return nil, err
	}

	remotes = make([]peers.Peer, 0, len(c.Peers)-1)

	for _, peer := range c.Peers {
		if local == peer.Name {
			continue
		}
		remotes = append(remotes, peer)
	}

	return remotes, nil
}

// GetPath searches possible configuration paths returning the first path it
// finds; this path is used when loading the configuration from disk. An
// error is returned if no configuration file exists.
func (c *Config) GetPath() (string, error) {
	// Prepare PATH list
	paths := make([]string, 0, 3)

	// Look in CWD directory first
	if path, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(path, "raft"))
	}

	// Look in user's home directory next
	if user, err := user.Current(); err == nil {
		paths = append(paths, filepath.Join(user.HomeDir, ".raft"))
	}

	// Finally look in etc for the global configuration
	paths = append(paths, "/etc/raft")

	for _, path := range paths {
		for _, ext := range []string{".toml", ".json", ".yml", ".yaml"} {
			fpath := path + ext
			if _, err := os.Stat(fpath); !os.IsNotExist(err) {
				return fpath, nil
			}
		}
	}

	return "", errors.New("no configuration file found")
}

// GetTick parses the tick duration and returns it.
func (c *Config) GetTick() (time.Duration, error) {
	return time.ParseDuration(c.Tick)
}

// GetTimeout parses the timeout duration and returns it.
func (c *Config) GetTimeout() (time.Duration, error) {
	return time.ParseDuration(c.Timeout)
}

// GetUptime parses the uptime duration and returns it.
func (c *Config) GetUptime() (time.Duration, error) {
	return time.ParseDuration(c.Uptime)
}

//===========================================================================
// Validators
//===========================================================================

// ComplexValidator validates complex types that multiconfig doesn't understand
type ComplexValidator struct {
	TagName string
}

// Validate implements the multiconfig.Validator interface.
func (v *ComplexValidator) Validate(s interface{}) error {
	if v.TagName == "" {
		v.TagName = "validate"
	}

	for _, field := range structs.Fields(s) {
		if err := v.processField("", field); err != nil {
			return err
		}
	}

	return nil
}

func (v *ComplexValidator) processField(fieldName string, field *structs.Field) error {
	fieldName += field.Name()
	switch field.Kind() {
	case reflect.Struct:
		fieldName += "."
		for _, f := range field.Fields() {
			if err := v.processField(fieldName, f); err != nil {
				return err
			}
		}
	default:
		if field.IsZero() {
			return nil
		}

		switch strings.ToLower(field.Tag(v.TagName)) {
		case "":
			return nil
		case "duration":
			return v.processDurationField(fieldName, field)
		case "url":
			return v.processURLField(fieldName, field)
		case "path":
			return v.processPathField(fieldName, field)
		case "uint":
			return v.processUintField(fieldName, field)
		default:
			return fmt.Errorf("cannot validate type '%s'", field.Tag(v.TagName))
		}

	}

	return nil
}

func (v *ComplexValidator) processDurationField(fieldName string, field *structs.Field) error {
	_, err := time.ParseDuration(field.Value().(string))
	if err != nil {
		return fmt.Errorf("could not validate %s: %s", fieldName, err.Error())
	}
	return nil
}

func (v *ComplexValidator) processURLField(fieldName string, field *structs.Field) error {
	if _, err := url.Parse(field.Value().(string)); err != nil {
		return fmt.Errorf("could not validate %s: %s", fieldName, err.Error())
	}

	return nil
}

func (v *ComplexValidator) processPathField(fieldName string, field *structs.Field) error {
	// No path validation quite yet
	return nil
}

func (v *ComplexValidator) processUintField(fieldName string, field *structs.Field) error {
	val := field.Value().(int)
	if val < 0 {
		return fmt.Errorf("%s is less than zero", fieldName)
	}
	return nil
}
