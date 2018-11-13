package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/empirefox/hybrid/pkg/zipfs"
)

var (
	root = flag.String("root", "", "root dir, which will be procceed.")
	out  = flag.String("out", "", "output file")
)

func main() {
	flag.Parse()

	if *root == "" {
		flag.PrintDefaults()
		log.Fatal("root should be set")
	}

	if *out == "" {
		*out = filepath.Join(filepath.Dir(*root), filepath.Base(*root)+".zip")
	}

	err := zipfs.GzipThenZip(*root, *out)
	if err != nil {
		log.Fatal(err)
	}
}
