package badgerutil

import (
	"github.com/dgraph-io/badger"
)

func NewBadger(path string, options *badger.Options) (*badger.DB, error) {
	var opt badger.Options
	if options == nil {
		opt = badger.DefaultOptions
	} else {
		opt = *options
	}
	opt.Dir = path
	opt.ValueDir = path

	return badger.Open(opt)
}
