package hybridclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/caarlos0/env"
	version "github.com/hashicorp/go-version"
	defaults "github.com/mcuadros/go-defaults"
	"github.com/tidwall/gjson"
	validator "gopkg.in/go-playground/validator.v9"
)

// TODO Need add config version migration func

const (
	ConfigVersion           = "1.0"
	ConfigVersionConstraint = "<=1.0"
)

func LoadConfig(c *Config) (*Config, error) {
	if c == nil {
		c = new(Config)
	}
	err := env.Parse(c)
	if err != nil {
		return nil, err
	}

	defaults.SetDefaults(c)

	t := NewConfigTree(c.BaseDir)
	configContent, err := ioutil.ReadFile(os.ExpandEnv(t.ConfigPath))
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
