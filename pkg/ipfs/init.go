package ipfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"

	"github.com/ipsn/go-ipfs/core"
	"github.com/ipsn/go-ipfs/namesys"
	"github.com/ipsn/go-ipfs/repo"
	"github.com/ipsn/go-ipfs/repo/fsrepo"
	logging "github.com/whyrusleeping/go-logging"

	cmdkit "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-cmdkit"
	config "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-config"
	ipfslog "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-log"
	logWriter "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-log/writer"
)

const (
	nBitsForKeypairDefault = 4096
)

// ForwardLog routes all ipfs logs to a file.
// Only messages >= INFO are logged.
func ForwardLog(w io.Writer) {
	logWriter.Configure(logWriter.Output(w))
	ipfslog.SetAllLoggers(logging.INFO)
}

// Fsck copied from RepoFsckCmd
func Fsck(repoRoot string) error {
	err := EnsureNoIpfs(repoRoot)
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

func InitDefaultOrMigrateRepoIfNeeded(c *Config) (repo.Repo, error) {
	err := InitWithDefaultsIfNotExist(c.RepoPath, c.Profile)
	if err != nil {
		return nil, err
	}

	// acquire the repo lock _before_ constructing a node. we need to make
	// sure we are permitted to access the resources (datastore, etc.)
	repo, err := fsrepo.Open(c.RepoPath)
	switch err {
	case fsrepo.ErrNeedMigration:
		if !c.AutoMigrate {
			return nil, err
		}

		err = MigrateDirectly(c.RepoPath, fsrepo.RepoVersion)
		if err != nil {
			return nil, err
		}

		repo, err = fsrepo.Open(c.RepoPath)
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

func InitWithDefaultsIfNotExist(repoRoot string, profile []string) error {
	_, err := os.Stat(repoRoot)
	if os.IsNotExist(err) {
		return InitWithDefaults(repoRoot, profile)
	}
	return err
}

func InitWithDefaults(repoRoot string, profile []string) error {
	return Init(repoRoot, nBitsForKeypairDefault, profile, nil)
}

func Init(repoRoot string, nBitsForKeypair int, confProfiles []string, conf *config.Config) error {
	err := EnsureNoIpfs(repoRoot)
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

	if err = fsrepo.Init(repoRoot, conf); err != nil {
		return err
	}

	if err = initializeCustomConfig(repoRoot); err != nil {
		return err
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

func initializeCustomConfig(repoRoot string) error {
	repo, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	swarmPort := findFreePortAfter(4001, 100)

	// Resource on the config keys can be found here:
	// https://github.com/ipfs/go-ipfs/blob/master/docs/config.md
	config := map[string]interface{}{
		"Addresses.Swarm": []string{
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", swarmPort),
			fmt.Sprintf("/ip6/::/tcp/%d", swarmPort),
		},
		"Addresses.API":     "",
		"Addresses.Gateway": "",
		"API.HTTPHeaders.Access-Control-Allow-Origin": []string{"*"},
		"Reprovider.Interval":                         "2h",
		"Swarm.ConnMgr.HighWater":                     200,
		"Swarm.ConnMgr.LowWater":                      100,
		"Swarm.EnableRelayHop":                        true,
	}

	for key, value := range config {
		if err := repo.SetConfigKey(key, value); err != nil {
			return err
		}
	}
	return nil
}

func initializeIpnsKeyspace(repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	hi, err := core.NewNode(ctx, &core.BuildCfg{Repo: repo})
	if err != nil {
		return err
	}
	defer hi.Close()

	err = hi.SetupOfflineRouting()
	if err != nil {
		return err
	}

	return namesys.InitializeKeyspace(ctx, hi.Namesys, hi.Pinning, hi.PrivateKey)
}

func EnsureNoIpfs(repoPath string) error {
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

// Find the next free tcp port near to `port` (possibly euqal to `port`).
// Only `maxTries` number of trials will be made.
// This method is (of course...) racy since the port might be already
// taken again by another process until we startup our service on that port.
func findFreePortAfter(port int, maxTries int) int {
	for idx := 0; idx < maxTries; idx++ {
		addr := fmt.Sprintf("localhost:%d", port+idx)
		lst, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}

		if err := lst.Close(); err != nil {
			// TODO: Well? Maybe do something?
		}

		return port + idx
	}

	return port
}
