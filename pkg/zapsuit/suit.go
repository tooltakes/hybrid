package zapsuit

import (
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/plimble/zap-sentry"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Dev bool

	Level *zapcore.Level

	// Target accepts "nop", "tcp://host:port?timeout=5s", filepath or sentryDSN.
	// Register NewTCPSink to support tcp sink. Default is stderr.
	Target string
}

func NewZap(config *Config, options ...zap.Option) (*zap.Logger, error) {
	if config.Target == "nop" {
		return zap.NewNop(), nil
	}

	var zcfg zap.Config
	if config.Dev {
		zcfg = zap.NewDevelopmentConfig()
	} else {
		zcfg = zap.NewProductionConfig()
	}

	if config.Level != nil {
		zcfg.Level.SetLevel(*config.Level)
	}

	u, err := url.Parse(config.Target)
	if err == nil && (u.Scheme == "https" || u.Scheme == "http") {
		switch u.Host {
		case "sentry.io":
			scfg := zapsentry.Configuration{DSN: config.Target}
			sentryCore, err := scfg.Build()
			if err != nil {
				return nil, err
			}
			sentryCoreFn := func(core zapcore.Core) zapcore.Core {
				return zapcore.NewTee(core, sentryCore)
			}
			options = append(options, zap.WrapCore(sentryCoreFn))
			return zcfg.Build(options...)
		default:
			return nil, fmt.Errorf("not implemented zapcore with url: %s", config.Target)
		}
	}

	if config.Target != "" {
		zcfg.OutputPaths = []string{config.Target}
		zcfg.ErrorOutputPaths = []string{config.Target}
	}
	return zcfg.Build(options...)
}

func RegisterTCPSink() error {
	return zap.RegisterSink("tcp", NewTCPSink)
}

func NewTCPSink(u *url.URL) (zap.Sink, error) {
	if u.User != nil {
		return nil, fmt.Errorf("zapsuit: user and password not allowed with file URLs: got %v", u)
	}
	if u.Fragment != "" {
		return nil, fmt.Errorf("zapsuit: fragments not allowed with file URLs: got %v", u)
	}
	if u.Path != "" {
		return nil, fmt.Errorf("zapsuit: path parameters not allowed with file URLs: got %v", u)
	}
	// Error messages are better if we check hostname and port separately.
	if u.Port() == "" {
		return nil, fmt.Errorf("zapsuit: port must be set with URLs: got %v", u)
	}

	query := u.Query()

	var dur time.Duration
	if timeout := query.Get("timeout"); timeout != "" {
		dur, err := time.ParseDuration(timeout)
		if err != nil {
			return nil, fmt.Errorf("zapsuit: parse timeout err: %v", err)
		}
		if dur < 0 {
			return nil, fmt.Errorf("zapsuit: timeout not allowed negative: got %v", timeout)
		}
	}

	conn, err := net.DialTimeout("tcp", u.Host, dur)
	if err != nil {
		return nil, fmt.Errorf("zapsuit: sink dial err: %v", err)
	}
	return &connSink{Conn: conn}, nil
}

type connSink struct {
	sync.Mutex
	net.Conn
}

func (s *connSink) Write(bs []byte) (int, error) {
	s.Lock()
	defer s.Unlock()
	return s.Conn.Write(bs)
}

func (s *connSink) Sync() error {
	return nil
}
