package config

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/caarlos0/env"
	"github.com/creasty/defaults"
	version "github.com/hashicorp/go-version"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/tidwall/gjson"
	validator "gopkg.in/go-playground/validator.v9"
)

// TODO Need add config version migration func

const (
	ConfigVersion           = "1"
	ConfigVersionConstraint = "=1"
)

func LoadConfig(rootPath string, c *Config) (*Config, error) {
	if rootPath == "" {
		rootPath = filepath.Join(homedir.Dir(), ".hybrid")
	}

	if c == nil {
		c = new(Config)
	}
	c.SetTree(nil)

	err := c.InitTree(rootPath)
	if err != nil {
		return nil, err
	}

	// 1. load env
	err = env.Parse(c)
	if err != nil {
		return nil, err
	}

	// 2. load default
	err = defaults.Set(c)
	if err != nil {
		return nil, err
	}

	configContent, err := ioutil.ReadFile(c.Tree().ConfigPath)
	if err != nil {
		return nil, err
	}

	// 3. check config version
	ver, err := version.NewVersion(gjson.GetBytes(configContent, "Version").String())
	if err != nil {
		return nil, err
	}

	constraints, err := version.NewConstraint(ConfigVersionConstraint)
	if err != nil {
		return nil, err
	}

	if !constraints.Check(ver) {
		return nil, fmt.Errorf("Current config version is v%s, need version '%s', but got v%s",
			ConfigVersion, ConfigVersionConstraint, ver)
	}

	// 4. unmarshal toml
	err = toml.Unmarshal(configContent, c)
	if err != nil {
		return nil, err
	}

	// 5. do struct validate
	validate := validator.New()
	err = validate.Struct(c)
	if err != nil {
		return nil, err
	}

	return c, nil
}
