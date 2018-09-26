package hybridclient

import (
	"path/filepath"

	defaults "github.com/mcuadros/go-defaults"
)

type ConfigTree struct {
	Version       string `default:"1.0"`
	RootBase      string
	RootName      string `default:".hybrid"`
	RootPath      string
	ConfigName    string `default:"hybrid.json"`
	ConfigPath    string
	FilesRootName string `default:"files-root"`
	FilesRootPath string
	RulesRootName string `default:"rules-root"`
	RulesRootPath string
}

func NewConfigTree(rootBase string) *ConfigTree {
	t := &ConfigTree{RootBase: rootBase}
	defaults.SetDefaults(t)
	t.RootPath = filepath.Join(t.RootBase, t.RootName)
	t.ConfigPath = filepath.Join(t.RootPath, t.ConfigName)
	t.FilesRootPath = filepath.Join(t.RootPath, t.FilesRootName)
	t.RulesRootPath = filepath.Join(t.RootPath, t.RulesRootName)
	return t
}
