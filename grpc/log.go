package grpc

import (
	"github.com/empirefox/hybrid/config"
	"github.com/empirefox/hybrid/pkg/zapsuit"
	"go.uber.org/zap"
)

func NewLogger(config *config.Config, options ...zap.Option) (*zap.Logger, error) {
	zapsuitConfig := zapsuit.Config{
		Dev:    config.Log.Dev,
		Target: config.Log.Target,
	}
	if config.Log.Level != "" {
		err := zapsuitConfig.Level.Set(config.Log.Level)
		if err != nil {
			return nil, err
		}
	}
	return zapsuit.NewZap(&zapsuitConfig, options...)
}
