package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/caarlos0/env"
	defaults "github.com/mcuadros/go-defaults"
	validator "gopkg.in/go-playground/validator.v9"
)

// tox-account
//{
//  "Address": "34402FB6A24AD8B0C520F08C125E71E0B44E4582E077F88A2A3CDE41C46070220F1769E6F8E1",
//  "Secret": "BC12FF844BCCE31EEA102611E8898C3107DE25101F3FE95039DBFB6DFEE29432",
//  "Pubkey": "34402FB6A24AD8B0C520F08C125E71E0B44E4582E077F88A2A3CDE41C4607022",
//  "Nospam": 253192678
//}

// hybrid-issuer
//ed25519 PrivateKey: ffa8b9066badac5d416b68132d6da5efbb495713ddc672b62e598919944faaebff1bc7523faa933eb936d7f127188b50894271b94e15a9e97d51516e1b06b6c6
//ed25519 PublicKey: ff1bc7523faa933eb936d7f127188b50894271b94e15a9e97d51516e1b06b6c6

type Verifier [32]byte

func (v *Verifier) VerifyKey(id uint32) ([]byte, bool) { return v[:], true }
func (v *Verifier) Revoked(id []byte) bool             { return false }

type Config struct {
	ScalarHex    string `validate:"len=64,hexadecimal"`
	VerifyKeyHex string `validate:"len=64,hexadecimal"`

	Config string `env:"HYBRID_TCP_SERVER_CONFIG" validate:"required" default:"$HOME/.hybrid/tcp-server.json" json:"-" yaml:"-" toml:"-"`
	Dev    bool   `env:"HYBRID_TCP_SERVER_DEV"`
	Port   uint16 `env:"HYBRID_TCP_SERVER_PORT"   validate:"gt=0"     default:"9999"`
}

func LoadConfig() (*Config, error) {
	c := new(Config)
	err := env.Parse(c)
	if err != nil {
		return nil, err
	}

	defaults.SetDefaults(c)

	configContent, err := ioutil.ReadFile(os.ExpandEnv(c.Config))
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
