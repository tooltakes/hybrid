package main

import (
	"log"
	"net/http"
	"os"

	"github.com/empirefox/hybrid/pkg/zipfs"
)

func main() {
	var name string
	if len(os.Args) > 1 {
		name = os.Args[1]
	} else {
		name = "public.zip"
	}

	hfs, closer, err := hybridzipfs.New(name)
	if err != nil {
		log.Fatal(err)
	}
	defer closer.Close()

	http.Handle("/", hfs)
	err = http.ListenAndServe(":9999", nil)
	if err != nil {
		log.Fatal(err)
	}
}
