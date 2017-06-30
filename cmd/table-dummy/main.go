package main

import (
	"log"
	"os"

	opensmtpd "gopkg.in/opensmtpd.v0"
)

var (
	proc = os.Args[0]
)

func main() {
	opensmtpd.Debug = true

	t := &opensmtpd.Table{
		// Update callback
		Update: func() (int, error) {
			log.Printf("%s: update")
			return 0, nil
		},

		// Check callback
		Check: func(service int, params opensmtpd.Dict, key string) (int, error) {
			log.Printf("%s: check service=%d, params=%+v, key=%q\n", proc, service, params, key)
			return 0, nil
		},

		// Lookup callback
		Lookup: func(service int, params opensmtpd.Dict, key string) (string, error) {
			log.Printf("%s: lookup service=%d,params=%+v,key=%q\n", proc, service, params, key)
			if key == "table-dummy-test" {
				return "maze@maze.io", nil
			}
			return "", nil
		},

		// Fetch callback
		Fetch: func(service int, params opensmtpd.Dict) (string, error) {
			log.Printf("%s: fetch service=%d,params=%+v,key=%q\n", proc, service, params)
			return "", nil
		},

		// Close callback, called at stop
		Close: func() error {
			log.Printf("%s: close\n")
			return nil
		},
	}

	if err := t.Serve(); err != nil {
		log.Fatalln(err)
	}
}
