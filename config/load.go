package hybridconfig

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/caarlos0/env"
	"github.com/creasty/defaults"
	version "github.com/hashicorp/go-version"
	"github.com/tidwall/gjson"
	validator "gopkg.in/go-playground/validator.v9"
)

// TODO Need add config version migration func

const (
	ConfigVersion           = "1"
	ConfigVersionConstraint = "=1"
)

func LoadConfig(c *Config) (*Config, error) {
	if c == nil {
		c = new(Config)
	}
	err := env.Parse(c)
	if err != nil {
		return nil, err
	}

	err = defaults.Set(c)
	if err != nil {
		return nil, err
	}

	t, err := c.ConfigTree()
	configContent, err := ioutil.ReadFile(t.ConfigPath)
	if err != nil {
		return nil, err
	}

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

	err = json.Unmarshal(configContent, c)
	if err != nil {
		return nil, err
	}

	validate := validator.New()
	err = validate.Struct(c)
	if err != nil {
		return nil, err
	}

	return c, nil
}
