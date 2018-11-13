package authstore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/empirefox/hybrid/pkg/bufpool"
	"github.com/golang/protobuf/proto"
)

var (
	ErrDBRequired     = errors.New("DB required")
	ErrPrefixRequired = errors.New("Prefix required")

	ErrKeyExpired   = errors.New("key expired")
	ErrInvalidKeyID = errors.New("invalid key id")
)

type Secure interface {
	Encrypt(plaintext []byte) (ciphertext []byte)
	Decrypt(ciphertext []byte) (plaintext []byte, err error)
}

type nonCrypto struct{}

func (nonCrypto) Encrypt(plaintext []byte) (ciphertext []byte)            { return plaintext }
func (nonCrypto) Decrypt(ciphertext []byte) (plaintext []byte, err error) { return ciphertext, nil }

type Config struct {
	// TODO refactor to use github.com/ipfs/go-datastore?
	DB     *badger.DB
	Prefix []byte

	BufferPool *bufpool.Pool
}

type KeyStore struct {
	db        *badger.DB
	prefix    []byte
	prefixLen int
	metaKey   []byte
	secure    Secure
	keypool   *bufpool.Pool
	valuepool *bufpool.Pool
}

func New(config *Config) (*KeyStore, error) {
	if config.DB == nil {
		return nil, ErrDBRequired
	}
	if len(config.Prefix) == 0 {
		return nil, ErrPrefixRequired
	}

	// for uint64
	const idLen = 8

	prefixLen := len(config.Prefix)
	metaKey := make([]byte, prefixLen+idLen)
	prefix := metaKey[:prefixLen]
	copy(prefix, config.Prefix)
	binary.BigEndian.PutUint64(metaKey[prefixLen:], 0)

	valuepool := config.BufferPool
	if valuepool == nil {
		valuepool = bufpool.Default1K
	}

	s := &KeyStore{
		db:        config.DB,
		prefix:    prefix,
		prefixLen: prefixLen,
		metaKey:   metaKey,
		secure:    nonCrypto{},
		keypool:   bufpool.NewSizeModify(prefixLen+idLen, func(b []byte) { copy(b, prefix) }),
		valuepool: valuepool,
	}
	return s, nil
}

// Secure must be called before any crud. Meta not affected.
func (s *KeyStore) Secure(secure Secure) {
	s.secure = secure
}

func (s *KeyStore) GetKey(keyid []byte) ([]byte, error) {
	if len(keyid) != 8 {
		return nil, ErrInvalidKeyID
	}

	ak, err := s.Find(binary.BigEndian.Uint64(keyid))
	if err != nil {
		return nil, err
	}

	if ak.ExpiresAt > time.Now().Unix() {
		return nil, ErrKeyExpired
	}

	return ak.Key, nil
}

func (s *KeyStore) Save(ak *AuthKey) (err error) {
	if ak.Id == 0 {
		ak.Id, err = s.NextId()
		if err != nil {
			return err
		}
	}
	pbuf, buf := s.getValueBuffer()
	defer s.putValueBuffer(buf)
	err = pbuf.Marshal(ak)
	if err != nil {
		return err
	}

	ciphertext := s.secure.Encrypt(pbuf.Bytes())

	key := s.keypool.Get()
	defer s.keypool.Put(key)
	binary.BigEndian.PutUint64(key[s.prefixLen:], ak.Id)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, ciphertext)
	})
}

func (s *KeyStore) Find(id uint64) (*AuthKey, error) {
	if id == 0 {
		return nil, ErrInvalidKeyID
	}

	key := s.keypool.Get()
	defer s.keypool.Put(key)
	binary.BigEndian.PutUint64(key[s.prefixLen:], id)

	cryptBuf := s.valuepool.Get()
	defer s.valuepool.Put(cryptBuf)

	ciphertext := cryptBuf[:0]
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		ciphertext, err = item.ValueCopy(ciphertext)
		return err
	})
	if err != nil {
		return nil, err
	}

	plaintext, err := s.secure.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}

	var ak AuthKey
	err = proto.Unmarshal(plaintext, &ak)
	if err != nil {
		return nil, err
	}

	return &ak, nil
}

func (s *KeyStore) Slice(start uint64, size int, reverse bool) (aks []*AuthKey, err error) {
	if start == 0 && !reverse {
		start = 1
	}

	valueBufs := make([][]byte, size)
	for i := 0; i < size; i++ {
		cryptBuf := s.valuepool.Get()
		defer s.valuepool.Put(cryptBuf)
		valueBufs[i] = cryptBuf[:0]
	}

	key := s.keypool.Get()
	defer s.keypool.Put(key)
	binary.BigEndian.PutUint64(key[s.prefixLen:], start)
	total := 0
	verr := s.db.View(func(txn *badger.Txn) (err error) {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = size
		opts.Reverse = reverse
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(key); it.ValidForPrefix(s.prefix); it.Next() {
			item := it.Item()
			if reverse && bytes.Equal(item.Key(), s.metaKey) {
				// ignote meta
				continue
			}
			valueBufs[total], err = item.ValueCopy(valueBufs[total])
			if err != nil {
				return err
			}
			total++
		}
		return nil
	})

	aks = make([]*AuthKey, 0, total)
	for i := 0; i < total; i++ {
		plaintext, err := s.secure.Decrypt(valueBufs[i])
		if err != nil {
			return aks, err
		}

		ak := new(AuthKey)
		err = proto.Unmarshal(plaintext, ak)
		if err != nil {
			return aks, err
		}
		aks = append(aks, ak)
	}
	return aks, verr
}

func (s *KeyStore) Delete(id uint64) error {
	if id == 0 {
		return ErrInvalidKeyID
	}

	key := s.keypool.Get()
	defer s.keypool.Put(key)
	binary.BigEndian.PutUint64(key[s.prefixLen:], id)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// SetMeta set user meta info, eg secure info.
func (s *KeyStore) SetMeta(meta []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(s.metaKey, meta)
	})
}

func (s *KeyStore) GetMeta() (meta []byte, err error) {
	s.db.View(func(txn *badger.Txn) error {
		var item *badger.Item
		item, err = txn.Get(s.metaKey)
		if err != nil {
			return err
		}
		meta, err = item.ValueCopy(nil)
		return nil
	})
	return
}

func (s *KeyStore) DeleteMeta() error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(s.metaKey)
	})
}

// NextId starts from 1, 0 is for meta.
func (s *KeyStore) NextId() (uint64, error) {
	seq, err := s.db.GetSequence(s.prefix, 1)
	if err != nil {
		return 0, err
	}
	defer seq.Release()
	num, err := seq.Next()
	if err != nil {
		return 0, err
	}
	if num != 0 {
		return num, err
	}
	return seq.Next()
}

func (s *KeyStore) getValueBuffer() (pbuf *proto.Buffer, buf []byte) {
	buf = s.valuepool.Get()
	pbuf = proto.NewBuffer(buf[:0])
	return
}

func (s *KeyStore) putValueBuffer(buf []byte) {
	s.valuepool.Put(buf)
}
