// reserved names: DIRECT localhost 127.0.0.1 0.0.0.0 0
// env:
// HYBRID_CONFIG_PARENT=$HOME
// HYBRID_DEV=false
// HYBRID_EXPOSE=7777
// HYBRID_FILE_SERVERS_DISABLED=a,b,c
// HYBRID_ROUTER_DISABLED=a,b,c
package hybridclient

// server types

type TcpServer struct {
	Name            string `validate:"omitempty,hostname"`
	Addr            string `validate:"tcp_addr"`
	NoTLS           bool
	ClientScalarHex string `validate:"omitempty,len=64,hexadecimal"`
	ServerPublicHex string `validate:"len=64,hexadecimal"`
	NoAuth          bool
	Token           string `validate:"lte=732"`
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
	Schema  string `json:"-" yaml:"-" toml:"-"`
	BaseDir string `env:"HYBRID_CONFIG_BSAE_DIR" validate:"required" default:"$HOME" json:"-" yaml:"-" toml:"-"`

	Version string
	Dev     bool   `env:"HYBRID_DEV"`
	Expose  uint16 `env:"HYBRID_EXPOSE"        validate:"gt=0"     default:"7777"`

	ScalarHex string `validate:"len=64,hexadecimal"`
	ToxNospam uint32 `validate:"required"`
	Token     string `validate:"required,lte=732"`

	TcpServers       []TcpServer
	FileServers      []FileServer
	HttpProxyServers []HttpProxyServer

	Routers []RouterItem
}
