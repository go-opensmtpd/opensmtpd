package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"

	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/hcl"
	opensmtpd "gopkg.in/opensmtpd.v52"
)

var (
	cache   *lru.Cache
	ignored []*net.IPNet
	config  struct {
		Cache  int
		Ignore []string
		Accept []string
		Reject []string
	}
)

func debugf(format string, args ...interface{}) {
	log.Printf("debug: "+format, args...)
}

func update() (int, error) {
	log.Println("table-rbl: update")
	return 1, nil
}

func reverse(ip net.IP) net.IP {
	log.Printf("ip: %#+v", ip)
	return net.IP{ip[3], ip[2], ip[1], ip[0]}
}

func lookup(rbl string, host net.IP) (result string, listed bool, err error) {
	var (
		query   = fmt.Sprintf("%s.%s", host, rbl)
		results []string
	)
	log.Printf("table-rbl: lookup %q", query)
	if results, err = net.LookupHost(query); err != nil {
		if strings.HasSuffix(err.Error(), ": no such host") {
			err = nil
		}
		return
	}

	if listed = len(results) > 0; listed {
		txts, _ := net.LookupTXT(query)
		if len(txts) > 0 {
			result = txts[0]
		}
	}
	return
}

func check(service int, params opensmtpd.Dict, key string) (int, error) {
	log.Printf("table-rbl: check key=%q", key)
	if key == "local" {
		return 1, nil
	}

	ips, err := net.LookupIP(key)
	if err != nil {
		log.Printf("table-rbl: error looking up %q: %v", key, err)
		return -1, err
	}

	for _, ip := range ips {
		if ip = ip.To4(); ip == nil {
			continue
		}
		log.Printf("table-rbl: %q resolved to %s (%s)", key, ip, reverse(ip))
		for _, network := range ignored {
			if network.Contains(ip) {
				log.Printf("table-rbl: %s is ignored", ip)
				return 1, nil
			}
		}

		if result, block := cache.Get(key); block && result.(string) != "" {
			log.Printf("table-rbl: reject %s (reason %q)", ip, result)
			return 0, nil
		}

		var (
			result string
			listed bool
			host   = reverse(ip)
			err    error
		)
		for _, rbl := range config.Accept {
			if result, listed, err = lookup(rbl, host); err != nil {
				log.Printf("table-rbl: error looking up %q in %q: %v", host, rbl, err)
				return -1, nil
			} else if listed {
				log.Printf("table-rbl: accept %q (reason %q)", ip, result)
				return 1, nil
			}
		}
		for _, rbl := range config.Reject {
			if result, listed, err = lookup(rbl, host); err != nil {
				log.Printf("table-rbl: error looking up %q in %q: %v", host, rbl, err)
				return -1, nil
			} else if listed {
				log.Printf("table-rbl: reject %q (reason %q)", ip, result)
				return 0, nil
			}
		}
	}

	log.Printf("table-rbl: accept %q (not rejected)", key)
	return 1, nil
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		panic(fmt.Sprintf("%s <config>\n", os.Args[0]))
	}
	log.Printf("table-rbl: args=%v", flag.Args())

	b, err := ioutil.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatalln("table-rbl", err)
	}
	if err = hcl.Unmarshal(b, &config); err != nil {
		log.Fatalln("table-rbl", err)
	}
	if len(config.Reject) == 0 {
		log.Fatalln("table-rbl: no reject rules configured")
	}

	// Setup cache
	if config.Cache == 0 {
		cache, err = lru.New(1024)
	} else {
		cache, err = lru.New(config.Cache)
	}
	if err != nil {
		log.Fatalln("table-rbl", err)
	}

	// Parse ignore rules
	for _, prefix := range config.Ignore {
		var ipnet *net.IPNet
		if _, ipnet, err = net.ParseCIDR(prefix); err != nil {
			panic(err)
		}
		ignored = append(ignored, ipnet)
		debugf("ignore %s", ipnet)
	}

	opensmtpd.Debug = true

	table := &opensmtpd.Table{
		Update: update,
		Check:  check,
		Close: func() error {
			log.Println("table-rbl: close")
			return nil
		},
	}
	log.Fatalln(table.Serve())
}
