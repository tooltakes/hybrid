package hybridconfig

import (
	"encoding/json"
	"io/ioutil"

	"github.com/caarlos0/env"
	defaults "github.com/mcuadros/go-defaults"
	validator "gopkg.in/go-playground/validator.v9"
)

func Load() (*Config, error) {
	c := new(Config)
	err := env.Parse(c)
	if err != nil {
		return nil, err
	}

	defaults.SetDefaults(c)

	configContent, err := ioutil.ReadFile(c.Config)
	if err != nil {
		return nil, err
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
