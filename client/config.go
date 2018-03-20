package hybrid

import (
	"github.com/empirefox/cement/clog"
)

type TcpClient struct {
	Addr            string `validate:"omitempty,url"`
	ServerPublicHex string
	ClientSecretHex string
}

type ToxClient struct {
	ServerAddressHex string
	Token            string
}

// routers

type AdpRouter struct {
	RouteOnOK bool
	Filenames []string
}

type LocalCDNRoute struct {
	LocalName string
	IPs       []string
	Nets      []string
}

type LocalCDNRouter struct {
	BaseDir string
	Routes  []LocalCDNRoute
}

type ToxRoute struct {
	Area string
	IPs  []string
	Nets []string
}

type ToxRouter struct {
	Routes []ToxrtcRoute
}

// routers end

type TLSConfig struct {
	SkipVerify bool
	ServerName string
}

type ClientConfig struct {
	Disabled    bool
	Exist       bool
	H2BufSizeKB int `default:"64"   validate:"gt=0"`

	// router
	AdpRouter      *AdpRouter
	LocalCDNRouter *LocalCDNRouter
	ToxrtcRouter   *ToxrtcRouter

	Dev bool
}

type Config struct {
	Schema string `json:"-" yaml:"-" toml:"-"`

	Expose int    `default:"7777" validate:"gt=0"`
	ResDir string `env:"H2PC_RES_DIR" default:"."`

	// create H2Client [h2, toxrtc]
	H2Toxrtc *H2Toxrtc

	Clients []ClientConfig
}

func (c *Config) GetEnvPtrs() []interface{} {
	return []interface{}{&c.H2Toxrtc, &c.Clog}
}
