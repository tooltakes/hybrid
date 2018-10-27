// reserved names: DIRECT localhost 127.0.0.1 0.0.0.0 0
// env:
// HYBRID_ROOT_PATH=$HOME/.hybrid
// HYBRID_DEV=false
// HYBRID_BIND=:7777
// HYBRID_FILE_SERVERS_DISABLED=a,b,c
// HYBRID_ROUTER_DISABLED=a,b,c
package hybridclient

import (
	"time"
)

const (
	HybridIpfsProtocolVersion = "1.0"
	HybridIpfsProtocol        = "/hybrid/1.0"
)

type Ipfs struct {
	ListenProtocols   []string `validate:"unique" default:"[\"/hybrid/1.0\"]"`
	FakeApiListenAddr string   `validate:"tcp_addr"`

	GatewayServerName string `validate:"omitempty,hostname"`
	ApiServerName     string `validate:"omitempty,hostname"`

	Profile          []string `validate:"unique"`
	AutoMigrate      bool
	EnableIPNSPubSub bool
	EnableFloodSub   bool
	EnableMultiplex  bool

	Token        string   `validate:"lte=732"`
	VerifyKeyHex string   `validate:"len=64,hexadecimal"`
	Revoked      []string `validate:"unique"`
}

// server types

type IpfsServer struct {
	Name     string `validate:"omitempty,hostname"`
	Peer     string `validate:"required"`
	Protocol string `validate:"required" default:"/hybrid/1.0"`
	Token    string `validate:"lte=732"`
}

type FileServer struct {
	Name        string `validate:"omitempty,hostname"`
	RootZipName string `validate:"hostname"`
	Redirect    map[string]string
	Dev         bool
}

type HttpProxyServer struct {
	Name      string `validate:"omitempty,hostname"`
	Host      string `validate:"tcp_addr"`
	KeepAlive bool
}

// server types end

// routers

type AdpRouter struct {
	B64RuleDirName      string `validate:"omitempty,hostname"`
	TxtRuleDirName      string `validate:"omitempty,hostname,nefield=B64RuleDirName"`
	Blocked             string `validate:"omitempty,hostname"`
	Unblocked           string `validate:"omitempty,hostname,nefield=Blocked"`
	EtcHostsIPAsBlocked bool
	Dev                 bool
}

type IPNetRouter struct {
	IPs       []string `validate:"dive,ip"`
	Nets      []string `validate:"dive,cidr"`
	Matched   string   `validate:"omitempty,hostname"`
	Unmatched string   `validate:"omitempty,hostname,nefield=Matched"`
	FileTest  string   `validate:"omitempty,hostname"`
}

// routers end

type RouterItem struct {
	Name string `validate:"omitempty,hostname"`
	// router
	Adp   *AdpRouter
	IPNet *IPNetRouter
}

type Config struct {
	Schema   string `json:"-" yaml:"-" toml:"-"`
	RootPath string `env:"HYBRID_ROOT_PATH" validate:"required" default:"$HOME/.hybrid" json:"-" yaml:"-" toml:"-"`

	Version string
	Dev     bool   `env:"HYBRID_DEV"`
	Bind    string `env:"HYBRID_BIND validate:"omitempty,tcp_addr"`

	ServerFlushInterval time.Duration `default:"200ms"`

	ScalarHex string `validate:"len=64,hexadecimal"`
	Token     string `validate:"omitempty,lte=732"`

	Ipfs Ipfs

	IpfsServers      []IpfsServer
	FileServers      []FileServer
	HttpProxyServers []HttpProxyServer

	Routers []RouterItem

	tree *ConfigTree
}
