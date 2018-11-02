package hybridconfig

import (
	"os"
	"path/filepath"

	"github.com/creasty/defaults"
)

type ConfigTree struct {
	Version       string `default:"1.0"`
	RootName      string
	RootPath      string
	ConfigName    string `default:"hybrid.json"`
	ConfigPath    string
	IpfsName      string `default:"ipfs"`
	IpfsPath      string
	FilesRootName string `default:"files-root"`
	FilesRootPath string
	RulesRootName string `default:"rules-root"`
	RulesRootPath string
}

func (c *Config) ConfigTree() (*ConfigTree, error) {
	if c.tree == nil {
		t, err := NewConfigTree(c.RootPath)
		if err != nil {
			return nil, err
		}
		c.tree = t
	}
	return c.tree, nil
}

func NewConfigTree(rootPath string) (*ConfigTree, error) {
	rootPath, err := filepath.Abs(os.ExpandEnv(rootPath))
	if err != nil {
		return nil, err
	}

	t := ConfigTree{
		RootName: filepath.Base(rootPath),
		RootPath: rootPath,
	}
	err = defaults.Set(&t)
	if err != nil {
		return nil, err
	}

	t.ConfigPath = filepath.Join(t.RootPath, t.ConfigName)
	t.IpfsPath = filepath.Join(t.RootPath, t.IpfsName)
	t.FilesRootPath = filepath.Join(t.RootPath, t.FilesRootName)
	t.RulesRootPath = filepath.Join(t.RootPath, t.RulesRootName)
	return &t, nil
}
