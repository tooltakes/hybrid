package hybridstore

import (
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/proto"
)

type RevokedToxStore struct {
	db     *bolt.DB
	bucket []byte

	m   map[string]*RevokedTox
	buf *proto.Buffer
	mu  sync.RWMutex
}

// TODO use config
func NewRevokedToxStore(db *bolt.DB) (*RevokedToxStore, error) {
	s := &RevokedToxStore{
		db:     db,
		bucket: []byte("RevokedTox"),
		m:      make(map[string]*RevokedTox),
		buf:    proto.NewBuffer(make([]byte, 4<<10)),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *RevokedToxStore) Revoked(id []byte) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.m[string(id)]
	return ok
}

func (s *RevokedToxStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(s.bucket)
		if err != nil {
			return err
		}

		return b.ForEach(func(k, v []byte) error {
			pb := new(RevokedTox)
			err := proto.Unmarshal(v, pb)
			if err != nil {
				return err
			}
			s.m[string(k)] = pb
			return nil
		})
	})
}

func (s *RevokedToxStore) Bytes() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := &RevokedToxes{M: s.m}
	err := s.buf.Marshal(m)
	buf := s.buf.Bytes()
	if buf == nil && err == nil {
		// Return a non-nil slice on success.
		return []byte{}, nil
	}
	return buf, err
}

func (s *RevokedToxStore) Save(id []byte, p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pb := new(RevokedTox)
	err := proto.Unmarshal(p, pb)
	if err != nil {
		return err
	}

	pb.CreatedAt = time.Now().Unix()
	err = s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(s.bucket).Put(id, p)
	})
	if err != nil {
		return err
	}

	s.m[string(id)] = pb
	return nil
}

func (s *RevokedToxStore) Delete(id []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(s.bucket).Delete(id)
	})
	if err != nil {
		return err
	}

	delete(s.m, string(id))
	return nil
}
