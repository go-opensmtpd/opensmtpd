package opensmtpd

import (
	"fmt"
	"os"
	"strings"
)

const (
	// FilterVersion is the supported filter API version
	FilterVersion = 52

	// QueueVersion is the supported queue API version
	QueueVersion = 2

	// TableVersion is the supported table API version
	TableVersion = 2
)

var (
	// Debug flag
	Debug bool

	prog = os.Args[0]
)

// Services
const (
	ServiceNone        = 0x000
	ServiceAlias       = 0x001
	ServiceDomain      = 0x002
	ServiceCredentials = 0x004
	ServiceNetaddr     = 0x008
	ServiceUserinfo    = 0x010
	ServiceSource      = 0x020
	ServiceMailaddr    = 0x040
	ServiceAddrname    = 0x080
	ServiceMailaddrMap = 0x100
	ServiceRelayHost   = 0x200
	ServiceString      = 0x400
	ServiceAny         = 0xfff
)

var serviceTypeName = map[int]string{
	ServiceAlias:       "alias",
	ServiceDomain:      "domain",
	ServiceCredentials: "credentials",
	ServiceNetaddr:     "netaddr",
	ServiceUserinfo:    "userinfo",
	ServiceSource:      "source",
	ServiceMailaddr:    "mailaddr",
	ServiceAddrname:    "addrname",
	ServiceMailaddrMap: "maddrmap",
	ServiceRelayHost:   "relayhost",
	ServiceString:      "string",
}

func serviceName(service int) string {
	var s []string
	for i := 1; i <= service; i <<= 1 {
		if k := service & i; k != 0 {
			s = append(s, serviceTypeName[k])
		}
	}
	return strings.Join(s, ",")
}

func debugf(format string, v ...interface{}) {
	if !Debug {
		return
	}

	line := strings.TrimSuffix(fmt.Sprintf(format, v...), "\n")
	fmt.Fprintln(os.Stderr, prog+": debug: "+line)
}

func fatal(v ...interface{}) {
	line := strings.TrimSuffix(fmt.Sprint(v...), "\n")
	fmt.Fprintln(os.Stderr, prog+": "+line)
	os.Exit(1)
}

func fatalf(format string, v ...interface{}) {
	line := strings.TrimSuffix(fmt.Sprintf(format, v...), "\n")
	fmt.Fprintln(os.Stderr, prog+": "+line)
	os.Exit(1)
}

const (
	maxLineSize = 2048
)

// Dict is a key-value mapping
type Dict map[string]interface{}
