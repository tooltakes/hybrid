package hybridconfig

// server types

type TcpServer struct {
	Name            string
	Addr            string `validate:"tcp_addr"`
	ServerPublicHex string `validate:"len=64,hexadecimal"`
	ClientSecretHex string `validate:"len=64,hexadecimal"`
}

type ToxServer struct {
	Name             string
	ServerAddressHex string `validate:"len=76,hexadecimal"`
	Token            string `validate:"required"`
}

type FileServer struct {
	Name        string
	DirName     string `validate:"base64"`
	StripPrefix string `validate:"omitempty,url"`
}

type HttpProxyServer struct {
	Name string
	Host string `validate:"tcp_addr"`
}

// server types end

// routers

type AdpRouter struct {
	B64RuleFiles []string `validate:"dive,uri"`
	TxtRuleFiles []string `validate:"dive,uri"`
	Blocked      string   `validate:"required"`
	Unblocked    string   `validate:"required,nefield=Blocked"`
}

type IPNetRouter struct {
	IPs       []string `validate:"dive,ip"`
	Nets      []string `validate:"dive,cidr"`
	Matched   string   `validate:"required"`
	Unmatched string   `validate:"required,nefield=Matched"`
}

// routers end

type RouterItem struct {
	SkipIfEnv string
	// router
	AdpRouter   *AdpRouter
	IPNetRouter *IPNetRouter
}

type Config struct {
	Schema string `json:"-" yaml:"-" toml:"-"`

	Config      string `env:"HYBRID_CONFIG"        validate:"uri"  default:"$HOME/.hybrid/hybrid.json" json:"-" yaml:"-" toml:"-"`
	Dev         bool   `env:"HYBRID_DEV"`
	Expose      uint16 `env:"HYBRID_EXPOSE"        validate:"gt=0" default:"7777"`
	FileRootDir string `env:"HYBRID_FILE_ROOT_DIR" validate:"uri"  default:"$HOME/.hybrid/file-root"`

	TcpServers       []TcpServer
	ToxServers       []ToxServer
	FileServers      []FileServer
	HttpProxyServers []HttpProxyServer

	Routers []RouterItem
}
