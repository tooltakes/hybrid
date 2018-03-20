package hybridstore

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/ed25519"

	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/proto"
)

type VerifyKeyStore struct {
	db     *bolt.DB
	bucket []byte

	m   map[uint32]*VerifyKey
	buf *proto.Buffer
	mu  sync.RWMutex
}

// TODO use config
func NewVerifyKeyStore(db *bolt.DB) (*VerifyKeyStore, error) {
	s := &VerifyKeyStore{
		db:     db,
		bucket: []byte("VerifyKey"),
		m:      make(map[uint32]*VerifyKey),
		buf:    proto.NewBuffer(make([]byte, 4<<10)),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *VerifyKeyStore) VerifyKey(id uint32) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sk, ok := s.m[id]; ok && sk.ExpiresAt > time.Now().Unix() {
		return sk.Key, true
	}
	return nil, false
}

func (s *VerifyKeyStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(s.bucket)
		if err != nil {
			return err
		}

		return b.ForEach(func(k, v []byte) error {
			pb := new(VerifyKey)
			err := proto.Unmarshal(v, pb)
			if err != nil {
				return err
			}
			s.m[binary.BigEndian.Uint32(k)] = pb
			return nil
		})
	})
}

func (s *VerifyKeyStore) Bytes() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := &VerifyKeys{M: s.m}
	err := s.buf.Marshal(m)
	buf := s.buf.Bytes()
	if buf == nil && err == nil {
		// Return a non-nil slice on success.
		return []byte{}, nil
	}
	return buf, err
}

func (s *VerifyKeyStore) Save(id uint32, p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pb := new(VerifyKey)
	err := proto.Unmarshal(p, pb)
	if err != nil {
		return err
	}
	if err = ValidateVerifyKey(pb); err != nil {
		return err
	}
	pb.CreatedAt = time.Now().Unix()

	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, id)
	err = s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(s.bucket).Put(key[:], p)
	})
	if err != nil {
		return err
	}

	s.m[id] = pb
	return nil
}

func (s *VerifyKeyStore) Delete(id uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, id)
	err := s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(s.bucket).Delete(key[:])
	})
	if err != nil {
		return err
	}

	delete(s.m, id)
	return nil
}

// TODO use config size
func ValidateVerifyKey(sk *VerifyKey) error {
	if len(sk.Key) != ed25519.PublicKeySize {
		return errors.New("key len not 32")
	}
	return nil
}
