package config

import (
	"os"
	"path/filepath"
)

func (c *Config) ConfigTree() (*ConfigTree, error) {
	if c.tree == nil {
		t, err := NewTree(c.RootPath)
		if err != nil {
			return nil, err
		}
		c.tree = t
	}
	return c.tree, nil
}

func NewTree(rootPath string) (*ConfigTree, error) {
	rootPath, err := filepath.Abs(os.ExpandEnv(rootPath))
	if err != nil {
		return nil, err
	}

	t := ConfigTree{
		Version:       "1.0",
		RootName:      filepath.Base(rootPath),
		RootPath:      rootPath,
		ConfigName:    "hybrid.json",
		IpfsName:      "ipfs",
		StoreName:     "store",
		FilesRootName: "files-root",
		RulesRootName: "rules-root",
	}

	t.ConfigPath = filepath.Join(t.RootPath, t.ConfigName)
	t.IpfsPath = filepath.Join(t.RootPath, t.IpfsName)
	t.StorePath = filepath.Join(t.RootPath, t.StoreName)
	t.FilesRootPath = filepath.Join(t.RootPath, t.FilesRootName)
	t.RulesRootPath = filepath.Join(t.RootPath, t.RulesRootName)
	return &t, nil
}
