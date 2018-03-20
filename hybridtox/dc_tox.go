package hybridtox

import (
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"strings"
	"time"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/toxenums"
	"go.uber.org/zap"
)

var (
	RequestRemoteSuperMessage = []byte("Master, please command me!")
)

func init() {
	log.SetFlags(log.Flags() | log.Lshortfile)
}

type ToxTCPConfig struct {
	Log *zap.Logger
	Tox *tox.Tox

	// DialSecond should at least 1 second
	DialSecond      time.Duration
	Supers          []*[tox.ADDRESS_SIZE]byte
	Servers         []*[tox.ADDRESS_SIZE]byte
	RequestToken    func(pubkey *[tox.PUBLIC_KEY_SIZE]byte) []byte
	ValidateRequest func(pubkey *[tox.PUBLIC_KEY_SIZE]byte, message []byte) bool

	// every friend may fire multi times
	OnFriendAddErr func(address *[tox.ADDRESS_SIZE]byte, err error)

	// controll tox only
	OnSupperAction func(friendNumber uint32, action []byte)
}

type friend struct {
	friendNumber uint32
	waiting      chan struct{}
	address      *[tox.ADDRESS_SIZE]byte
	pubkey       *[tox.PUBLIC_KEY_SIZE]byte
	isServer     bool
	isSuper      bool
	online       bool
}

type ToxTCP struct {
	config          ToxTCPConfig
	isSuperByPubkey map[[tox.PUBLIC_KEY_SIZE]byte]bool
	pkToAddr        map[[tox.PUBLIC_KEY_SIZE]byte]*[tox.ADDRESS_SIZE]byte
	onlineWaiting   map[uint32]chan struct{}
	num2Friends     map[uint32]*friend
	pk2Friends      map[[tox.PUBLIC_KEY_SIZE]byte]*friend
}

func NewToxTCP(config ToxTCPConfig) *ToxTCP {
	pure := &ToxTCP{
		config:          config,
		isSuperByPubkey: make(map[[tox.PUBLIC_KEY_SIZE]byte]bool),
		pkToAddr:        make(map[[tox.PUBLIC_KEY_SIZE]byte]*[tox.ADDRESS_SIZE]byte),
		onlineWaiting:   make(map[uint32]chan struct{}),
		num2Friends:     make(map[uint32]*friend),
		pk2Friends:      make(map[[tox.PUBLIC_KEY_SIZE]byte]*friend),
	}
	for _, address := range config.Supers {
		pubkey := tox.ToPubkey(address)
		pure.isSuperByPubkey[*pubkey] = true
		pure.pkToAddr[*pubkey] = address
		pure.pk2Friends[*pubkey] = &friend{
			address: address,
			pubkey:  pubkey,
			isSuper: true,
		}
	}
	for _, address := range config.Servers {
		pubkey := tox.ToPubkey(address)
		pure.pkToAddr[*pubkey] = address
		pure.pk2Friends[*pubkey] = &friend{
			address:  address,
			pubkey:   pubkey,
			isServer: true,
		}
	}

	t := config.Tox
	t.CallbackSelfConnectionStatus(pure.onSelfConnectionStatus)
	t.CallbackFriendRequest(pure.onFriendRequest)
	t.CallbackFriendMessage(pure.onFriendMessage)
	t.CallbackFriendConnectionStatus(pure.onFriendConnectionStatus)
	t.CallbackFriendLosslessPacket(t.ParseLosslessPacket)
	t.CallbackTcpPong(pure.onTcpPong)
	return pure
}

func (pure *ToxTCP) Create() (l net.Listener, err error) {
	return pure.config.Tox, nil
}

func (pure *ToxTCP) Dial(addr string) (net.Conn, []string, error) {
	addr = strings.SplitN(addr, ":", 2)[0]
	address, err := tox.DecodeAddress(addr)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*pure.config.DialSecond)
	defer cancel()
	c, err := pure.DialContext(ctx, address)
	return c, nil, err
}

func (pure *ToxTCP) DialContext(ctx context.Context, address *[tox.ADDRESS_SIZE]byte) (net.Conn, error) {
	pubkey := tox.ToPubkey(address)

	t := pure.config.Tox
	var friend *friend
	var friendNumber uint32
	var ok bool
	var err error
	waiting := make(chan struct{}, 1)
	done := make(chan struct{}, 1)
	t.DoInLoop(func() {
		defer close(done)

		friend, ok = pure.pk2Friends[*pubkey]
		if !ok || !friend.isServer {
			err = fmt.Errorf("%X is not a known server", pubkey[:])
			return
		}

		friendNumber, ok = t.FriendByPublicKey(pubkey)
		if !ok {
			// not a friend yet
			friendNumber, err = t.FriendAdd_l(address, pure.config.RequestToken(pubkey))
			if err == nil {
				pure.onServerAdded(friend, friendNumber, waiting)
			}
			return
		}

		// already online, no need waiting
		if friend.online {
			close(waiting)
			return
		}

		t.FriendDelete_l(friendNumber)
		pure.onServerDeleted(friend)
		t.Iterate_l()

		friendNumber, err = t.FriendAdd_l(address, pure.config.RequestToken(pubkey))
		if err == nil {
			pure.onServerAdded(friend, friendNumber, waiting)
		}
	})
	<-done
	if err != nil {
		return nil, err
	}

	pure.config.Log.Info("waiting server add me...")
	select {
	case <-waiting:
	case <-ctx.Done():
		t.DoInLoop(func() {
			t.FriendDelete_l(friendNumber)
			pure.onServerDeleted(friend)
		})
		return nil, ctx.Err()
	}

	done = make(chan struct{}, 1)
	var c net.Conn
	t.DoInLoop(func() {
		c, err = t.Dial_l(friendNumber)
		close(done)
	})
	<-done
	return c, err
}

func (pure *ToxTCP) onSelfConnectionStatus(status toxenums.TOX_CONNECTION) {
	pure.config.Log.Info("onSelfConnectionStatus", zap.Stringer("status", status))
	if status != toxenums.TOX_CONNECTION_NONE {
		for _, address := range pure.config.Supers {
			_, err := pure.config.Tox.FriendAdd_l(address, RequestRemoteSuperMessage)
			if err != nil && pure.config.OnFriendAddErr != nil {
				if terr, ok := err.(toxenums.TOX_ERR_FRIEND_ADD); !ok || terr != toxenums.TOX_ERR_FRIEND_ADD_ALREADY_SENT {
					pure.config.OnFriendAddErr(address, err)
				}
			}
		}
	} else {
		var toDelete []uint32
		for _, address := range pure.config.Supers {
			friendNumber, ok := pure.config.Tox.FriendByPublicKey(tox.ToPubkey(address))
			if ok {
				toDelete = append(toDelete, friendNumber)
			}
		}
		pure.config.Tox.CallbackPostIterateOnce_l(func() time.Duration {
			for _, friendNumber := range toDelete {
				pure.config.Tox.FriendDelete_l(friendNumber)
			}
			return 0
		})
	}
}

func (pure *ToxTCP) onFriendRequest(pubkey *[tox.PUBLIC_KEY_SIZE]byte, message []byte) {
	pure.config.Log.Info("onFriendRequest", zap.ByteString("message", message))
	if _, ok := pure.pk2Friends[*pubkey]; ok {
		pure.config.Tox.FriendAddNorequest_l(pubkey)
		return
	}
	if pure.config.ValidateRequest(pubkey, message) {
		pure.config.Tox.FriendAddNorequest_l(pubkey)
	}
}

func (pure *ToxTCP) onFriendMessage(friendNumber uint32, mtype toxenums.TOX_MESSAGE_TYPE, message []byte) {
	// TODO refactor super to work with qtox/utox
	if pure.config.OnSupperAction != nil {
		if mtype == toxenums.TOX_MESSAGE_TYPE_ACTION {
			pubkey, ok := pure.config.Tox.FriendGetPublicKey(friendNumber)
			if ok && pure.isSuperByPubkey[*pubkey] {
				pure.config.OnSupperAction(friendNumber, message)
			}
		}
	}
}

func (pure *ToxTCP) onFriendConnectionStatus(friendNumber uint32, status toxenums.TOX_CONNECTION) {
	pure.config.Log.Info("onFriendConnectionStatus", zap.Uint32("friendNumber", friendNumber), zap.Stringer("status", status))

	friend, ok := pure.num2Friends[friendNumber]
	if ok && friend.isSuper && status == toxenums.TOX_CONNECTION_NONE {
		// TODO report stats
	}

	if ok && friend.isServer {
		if status == toxenums.TOX_CONNECTION_NONE {
			// MUST do so, since it will use the friendNumber before iterate end.
			pure.config.Tox.CallbackPostIterateOnce_l(func() time.Duration {
				pure.config.Tox.SetPingMultiple_l(friendNumber, -1)
				friend.online = false
				pure.config.Tox.FriendDelete_l(friendNumber)
				pure.onServerDeleted(friend)
				return 0
			})
		} else if friend.waiting != nil {
			pure.config.Tox.SetPingMultiple_l(friendNumber, 10)
			friend.online = true
			close(friend.waiting)
			friend.waiting = nil
		}
	}

}

func (pure *ToxTCP) onTcpPong(friendNumber uint32, ms uint32) {
	friend, ok := pure.num2Friends[friendNumber]
	if ok {
		pure.config.Log.Info("PING back", zap.String("pubkey", fmt.Sprintf("%X", friend.pubkey[:4])), zap.Uint32("ms", ms))
	} else {
		pure.config.Log.Info("PING back", zap.Uint32("friendNumber", friendNumber), zap.Uint32("ms", ms))
	}
}

func (pure *ToxTCP) onServerAdded(friend *friend, friendNumber uint32, waiting chan struct{}) {
	pure.num2Friends[friendNumber] = friend
	friend.friendNumber = friendNumber
	friend.waiting = waiting
}

func (pure *ToxTCP) onServerDeleted(friend *friend) {
	delete(pure.num2Friends, friend.friendNumber)
	friend.friendNumber = math.MaxUint32
	if friend.waiting != nil {
		close(friend.waiting)
		friend.waiting = nil
	}
}
