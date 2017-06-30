package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	lru "github.com/hashicorp/golang-lru"

	"gopkg.in/opensmtpd.v0"
)

var (
	prog = os.Args[0]
	skip = []*net.IPNet{}
	rbls = []string{
		"b.barracudacentral.org",
		"bl.spamcop.net",
		"virbl.bit.nl",
		"xbl.spamhaus.org",
	}
	debug bool
	masq  bool
	cache *lru.Cache
)

func debugf(fmt string, args ...interface{}) {
	if !debug {
		return
	}
	log.Printf("debug: "+fmt, args...)
}

func reverse(ip net.IP) string {
	if ip.To4() == nil {
		return ""
	}

	splitAddress := strings.Split(ip.String(), ".")

	for i, j := 0, len(splitAddress)-1; i < len(splitAddress)/2; i, j = i+1, j-1 {
		splitAddress[i], splitAddress[j] = splitAddress[j], splitAddress[i]
	}

	return strings.Join(splitAddress, ".")
}

func lookup(rbl string, host string) (result string, listed bool, err error) {
	host = fmt.Sprintf("%s.%s", host, rbl)

	var res []string
	res, err = net.LookupHost(host)
	if listed = len(res) > 0; listed {
		txt, _ := net.LookupTXT(host)
		if len(txt) > 0 {
			result = txt[0]
		}
	}

	// Expected error
	if err != nil && strings.HasSuffix(err.Error(), ": no such host") {
		err = nil
	}

	return
}

func onConnect(s *opensmtpd.Session, query *opensmtpd.ConnectQuery) error {
	ip := query.Remote.(opensmtpd.Sockaddr).IP()
	if ip == nil {
		return nil
	}

	debugf("%s: connect from %s\n", prog, ip)

	for _, ipnet := range skip {
		if ipnet.Contains(ip) {
			debugf("%s: skip %s, IP ignored", prog, ip)
			return s.Accept()
		}
	}

	var (
		result string
		listed bool
		host   = reverse(ip)
		err    error
	)
	for _, rbl := range rbls {
		if result, listed, err = lookup(rbl, host); err != nil {
			log.Printf("%s: %s failed %s: %v\n", prog, rbl, ip, err)
		} else if listed {
			log.Printf("%s: %s listed %s: %v\n", prog, rbl, ip, result)
			cache.Add(s.ID, result)
			break
		}
	}

	debugf("%s: pass: %s\n", prog, ip)

	if !listed {
		// Add negative hit
		cache.Add(s.ID, "")
	}

	return s.Accept()
}

func onDATA(s *opensmtpd.Session) error {
	debugf("%s: %s DATA\n", prog, s)

	if result, block := cache.Get(s.ID); block && result.(string) != "" {
		return s.RejectCode(opensmtpd.FilterClose, 421, result.(string))
	}

	return s.Accept()
}

func main() {
	cacheSize := flag.Int("cache-size", 1024, "LRU cache size")
	rblServer := flag.String("servers", strings.Join(rbls, ","), "RBL servers")
	ignoreIPs := flag.String("ignore", "127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,fe80::/64", "ignore IPs")
	debugging := flag.Bool("d", false, "be verbose")
	verbosity := flag.Bool("v", false, "be verbose")
	flag.BoolVar(&masq, "masq", true, "masquerade SMTP banner")
	flag.Parse()

	debug = *debugging || *verbosity

	var err error
	if cache, err = lru.New(*cacheSize); err != nil {
		log.Fatalln(err)
	}

	rbls = strings.Split(*rblServer, ",")

	for _, prefix := range strings.Split(*ignoreIPs, ",") {
		var ipnet *net.IPNet
		if _, ipnet, err = net.ParseCIDR(prefix); err != nil {
			log.Fatalln(err)
		}
		skip = append(skip, ipnet)
		debugf("ignore: %s\n", ipnet)
	}

	filter := &opensmtpd.Filter{
		Connect: onConnect,
		DATA:    onDATA,
	}

	if err = filter.Register(); err != nil {
		log.Fatalln(err)
	}

	if err = filter.Serve(); err != nil {
		log.Fatalln(err)
	}
}
