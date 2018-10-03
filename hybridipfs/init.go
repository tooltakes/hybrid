package hybridipfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	assets "github.com/ipsn/go-ipfs/assets"
	"github.com/ipsn/go-ipfs/core"
	core "github.com/ipsn/go-ipfs/core"
	namesys "github.com/ipsn/go-ipfs/namesys"
	repo "github.com/ipsn/go-ipfs/repo"
	fsrepo "github.com/ipsn/go-ipfs/repo/fsrepo"
	migrate "github.com/ipsn/go-ipfs/repo/fsrepo/migrations"
	logging "github.com/whyrusleeping/go-logging"

	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-config"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-log"
	logWriter "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-log/writer"
)

const (
	nBitsForKeypairDefault = 4096
)

// ForwardLog routes all ipfs logs to a file provided by brig.
// Only messages >= INFO are logged.
func ForwardLog(w io.Writer) {
	logWriter.Configure(logWriter.Output(w))
	log.SetAllLoggers(logging.INFO)
}

// Fsck copied from RepoFsckCmd
func Fsck(repoRoot string) error {
	err := EnsureNoIpfsDaemon(repoRoot)
	if err != nil {
		return err
	}

	dsPath, err := config.DataStorePath(repoRoot)
	if err != nil {
		return err
	}

	dsLockFile := filepath.Join(dsPath, "LOCK") // TODO: get this lockfile programmatically
	repoLockFile := filepath.Join(repoRoot, fsrepo.LockFile)
	apiFile := filepath.Join(repoRoot, "api") // TODO: get this programmatically

	err = os.Remove(repoLockFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	err = os.Remove(dsLockFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	err = os.Remove(apiFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

var errRepoExists = errors.New(`ipfs configuration file already exists!`)

func InitDefaultOrMigrateRepoIfNeeded(repoPath string, opts *DaemonOptions) (repo.Repo, error) {
	err := InitWithDefaultsIfNotExist(repoPath, opts.Profile)
	if err != nil {
		return nil, err
	}

	// acquire the repo lock _before_ constructing a node. we need to make
	// sure we are permitted to access the resources (datastore, etc.)
	repo, err := fsrepo.Open(repoPath)
	switch err {
	case fsrepo.ErrNeedMigration:
		if !opts.AutoMigrate {
			return nil, err
		}

		err = migrate.RunMigration(fsrepo.RepoVersion)
		if err != nil {
			return nil, err
		}

		repo, err = fsrepo.Open(repoPath)
		if err != nil {
			return nil, err
		}
	case nil:
		break
	default:
		return nil, err
	}

	return repo, err
}

func InitWithDefaultsIfNotExist(repoRoot string, profile string) error {
	_, err := os.Stat(repoRoot)
	if os.IsNotExist(err) {
		return InitWithDefaults(repoRoot, profile)
	}
	return err
}

func InitWithDefaults(repoRoot string, profile string) error {
	var profiles []string
	if profile != "" {
		profiles = strings.Split(profile, ",")
	}
	return Init(repoRoot, false, nBitsForKeypairDefault, profiles, nil)
}

func Init(repoRoot string, empty bool, nBitsForKeypair int, confProfiles []string, conf *config.Config) error {
	err := EnsureNoIpfsDaemon(repoRoot)
	if err != nil {
		return err
	}

	err = checkWritable(repoRoot)
	if err != nil {
		return err
	}

	if fsrepo.IsInitialized(repoRoot) {
		return errRepoExists
	}

	if conf == nil {
		var err error
		conf, err = config.Init(ioutil.Discard, nBitsForKeypair)
		if err != nil {
			return err
		}
	}

	for _, profile := range confProfiles {
		transformer, ok := config.Profiles[profile]
		if !ok {
			return fmt.Errorf("invalid configuration profile: %s", profile)
		}

		if err := transformer.Transform(conf); err != nil {
			return err
		}
	}

	if err := fsrepo.Init(repoRoot, conf); err != nil {
		return err
	}

	if !empty {
		if err := addDefaultAssets(repoRoot); err != nil {
			return err
		}
	}

	return initializeIpnsKeyspace(repoRoot)
}

func checkWritable(dir string) error {
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}

	_, err = os.Stat(dir)
	if err == nil {
		// dir exists, make sure we can write to it
		testfile := path.Join(dir, "test")
		fi, err := os.Create(testfile)
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("%s is not writeable by the current user", dir)
			}
			return fmt.Errorf("unexpected error while checking writeablility of repo root: %s", err)
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// dir doesn't exist, check that we can create it
		return os.Mkdir(dir, 0775)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("cannot write to %s, incorrect permissions", err)
	}

	return err
}

func addDefaultAssets(repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	nd, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return err
	}
	defer nd.Close()

	dkey, err := assets.SeedInitDocs(nd)
	if err != nil {
		return fmt.Errorf("init: seeding init docs failed: %s", err)
	}
	return err
}

func initializeIpnsKeyspace(repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	nd, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return err
	}
	defer nd.Close()

	err = nd.SetupOfflineRouting()
	if err != nil {
		return err
	}

	return namesys.InitializeKeyspace(ctx, nd.Namesys, nd.Pinning, nd.PrivateKey)
}

func EnsureNoIpfsDaemon(repoPath string) error {
	// Ipfs instance will lock it
	daemonLocked, err := fsrepo.LockedByOtherProcess(repoPath)
	if err != nil {
		return err
	}

	if daemonLocked {
		e := "ipfs daemon is running. please stop it to run this command"
		return cmdkit.Error{Message: e}
	}

	return nil
}
