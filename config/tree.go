package config

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	ErrConfigTreeAlreadyInited = errors.New("config tree already inited")
)

func (c *Config) InitTree(rootPath string) error {
	if c.Tree() != nil {
		return ErrConfigTreeAlreadyInited
	}

	t, err := NewTree(rootPath)
	if err != nil {
		return err
	}

	c.SetTree(t)
	return nil
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
