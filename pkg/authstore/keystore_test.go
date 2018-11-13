package authstore

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger"
)

func newDB(t *testing.T) (*badger.DB, func()) {
	path, err := ioutil.TempDir("/tmp", "testing_badger_")
	if err != nil {
		t.Fatal(err)
	}
	opt := badger.DefaultOptions
	opt.Dir = path
	opt.ValueDir = path

	db, err := badger.Open(opt)
	if err != nil {
		t.Fatal(err)
	}

	return db, func() {
		db.Close()
		os.RemoveAll(path)
	}
}

func TestKeystore(t *testing.T) {
	db, cancel := newDB(t)
	defer cancel()

	config := &Config{
		DB:     db,
		Prefix: []byte("a/"),
	}
	s, err := New(config)
	if err != nil {
		t.Fatalf("New should get no err, but got: %v", err)
	}

	// test with no data
	_, err = s.Find(1)
	if err == nil {
		t.Fatalf("Find should get err when no data, but got nil")
	}

	aks, err := s.Slice(1, 100, false)
	if err != nil {
		t.Errorf("Slice should get no err when no data, but got: %v", err)
	}
	if len(aks) != 0 {
		t.Fatalf("Slice should get empty result when no data")
	}

	// save first one
	ak := &AuthKey{
		Tags:      []string{"first_key"},
		Key:       []byte("key1"),
		CreatedAt: time.Now().Unix(),
	}
	err = s.Save(ak)
	if err != nil {
		t.Fatalf("Save new ak should get no err, but got: %v", err)
	}
	if ak.Id != 1 {
		t.Fatalf("Save new ak should create id with 1, but got: %d", ak.Id)
	}

	// save second one
	ak = &AuthKey{
		Tags:      []string{"second_key"},
		Key:       []byte("key2"),
		CreatedAt: time.Now().Unix(),
	}
	err = s.Save(ak)
	if err != nil {
		t.Fatalf("Save new ak should get no err, but got: %v", err)
	}
	if ak.Id != 2 {
		t.Fatalf("Save new ak should create id with 2, but got: %d", ak.Id)
	}

	// Slice
	aks, err = s.Slice(1, 100, false)
	if err != nil {
		t.Errorf("Slice should get no err, but got: %v", err)
	}
	if len(aks) != 2 {
		t.Fatalf("Slice should get 2 result, but got: %d", len(aks))
	}

	// Find
	ak, err = s.Find(2)
	if err != nil {
		t.Fatalf("Find should get no err, but got: %v", err)
	}

	if ak.Tags[0] != aks[1].Tags[0] {
		t.Fatalf("Find should get second ak: %v, but got: %v", aks[1].Tags[0], ak.Tags[0])
	}
}
