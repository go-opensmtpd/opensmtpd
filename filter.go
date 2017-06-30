package opensmtpd

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	lru "github.com/hashicorp/golang-lru"
)

const (
	typeFilterRegister uint32 = iota
	typeFilterEvent
	typeFilterquery
	typeFilterPipe
	typeFilterResponse
)

var filterTypeName = map[uint32]string{
	typeFilterRegister: "IMSG_FILTER_REGISTER",
	typeFilterEvent:    "IMSG_FILTER_EVENT",
	typeFilterquery:    "IMSG_FILTER_QUERY",
	typeFilterPipe:     "IMSG_FILTER_PIPE",
	typeFilterResponse: "IMSG_FILTER_RESPONSE",
}

func filterName(t uint32) string {
	if s, ok := filterTypeName[t]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN %d", t)
}

const (
	hookConnect = 1 << iota
	hookHELO
	hookMAIL
	hookRCPT
	hookDATA
	hookEOM
	hookReset
	hookDisconnect
	hookCommit
	hookRollback
	hookDataLine
)

var hookTypeName = map[uint16]string{
	hookConnect:    "HOOK_CONNECT",
	hookHELO:       "HOOK_HELO",
	hookMAIL:       "HOOK_MAIL",
	hookRCPT:       "HOOK_RCPT",
	hookDATA:       "HOOK_DATA",
	hookEOM:        "HOOK_EOM",
	hookReset:      "HOOK_RESET",
	hookDisconnect: "HOOK_DISCONNECT",
	hookCommit:     "HOOK_COMMIT",
	hookRollback:   "HOOK_ROLLBACK",
	hookDataLine:   "HOOK_DATALINE",
}

func hookName(h uint16) string {
	var s []string
	for i := uint(0); i < 11; i++ {
		if h&(1<<i) != 0 {
			s = append(s, hookTypeName[(1<<i)])
		}
	}
	return strings.Join(s, ",")
}

const (
	eventConnect = iota
	eventReset
	eventDisconnect
	eventTXBegin
	eventTXCommit
	eventTXRollback
)

var eventTypeName = map[int]string{
	eventConnect:    "EVENT_CONNECT",
	eventReset:      "EVENT_RESET",
	eventDisconnect: "EVENT_DISCONNECT",
	eventTXBegin:    "EVENT_TX_BEGIN",
	eventTXCommit:   "EVENT_TX_COMMIT",
	eventTXRollback: "EVENT_TX_ROLLBACK",
}

func eventName(t int) string {
	if s, ok := eventTypeName[t]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN %d", int(t))
}

const (
	queryConnect = iota
	queryHELO
	queryMAIL
	queryRCPT
	queryDATA
	queryEOM
	queryDataLine
)

var queryTypeName = map[int]string{
	queryConnect:  "QUERY_CONNECT",
	queryHELO:     "QUERY_HELO",
	queryMAIL:     "QUERY_MAIL",
	queryRCPT:     "QUERY_RCPT",
	queryDATA:     "QUERY_DATA",
	queryEOM:      "QUERY_EOM",
	queryDataLine: "QUERY_DATALINE",
}

func queryName(t int) string {
	if s, ok := queryTypeName[t]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN %d", int(t))
}

const (
	FilterOK = iota
	FilterFail
	FilterClose
)

var responseTypeName = map[int]string{
	FilterOK:    "FILTER_OK",
	FilterFail:  "FILTER_FAIL",
	FilterClose: "FILTER_CLOSE",
}

func responseName(c int) string {
	if s, ok := responseTypeName[c]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN %d", c)
}

// Filter implements the OpenSMTPD filter API
type Filter struct {
	// Connect callback
	Connect func(*Session, *ConnectQuery) error

	// HELO callback
	HELO func(*Session, string) error

	// MAIL FROM callback
	MAIL func(*Session, string, string) error

	// RCPT TO callback
	RCPT func(*Session, string, string) error

	// DATA callback
	DATA func(*Session) error

	// DataLine callback
	DataLine func(*Session, string) error

	// EOM (end of message) callback
	EOM func(*Session, uint32) error

	// Reset callback
	Reset func(*Session) error

	// Disconnect callback
	Disconnect func(*Session) error

	// Commit callback
	Commit func(*Session) error

	Name    string
	Version uint32

	c net.Conn
	m *message

	hooks   int
	flags   int
	ready   bool
	session *lru.Cache
}

// Register our filter with OpenSMTPD
func (f *Filter) Register() error {
	var err error
	if f.m == nil {
		f.m = new(message)
	}
	if f.c == nil {
		if f.c, err = newConn(0); err != nil {
			return err
		}
	}
	if err = f.m.ReadFrom(f.c); err != nil {
		return err
	}

	// Fill hooks mask
	if f.Connect != nil {
		f.hooks |= hookConnect
	}
	if f.HELO != nil {
		f.hooks |= hookHELO
	}
	if f.MAIL != nil {
		f.hooks |= hookMAIL
	}
	if f.RCPT != nil {
		f.hooks |= hookRCPT
	}
	if f.DATA != nil {
		f.hooks |= hookDATA
	}
	if f.DataLine != nil {
		f.hooks |= hookDataLine
	}
	if f.EOM != nil {
		f.hooks |= hookEOM
	}
	if f.Disconnect != nil {
		f.hooks |= hookDisconnect
	}
	if f.Commit != nil {
		f.hooks |= hookCommit
	}

	if t, ok := filterTypeName[f.m.Type]; ok {
		log.Printf("filter: imsg %s\n", t)
	} else {
		log.Printf("filter: imsg UNKNOWN %d\n", f.m.Type)
	}

	switch f.m.Type {
	case typeFilterRegister:
		var err error
		if f.Version, err = f.m.GetTypeUint32(); err != nil {
			return err
		}
		if f.Name, err = f.m.GetTypeString(); err != nil {
			return err
		}
		log.Printf("register version=%d,name=%q\n", f.Version, f.Name)

		f.m.reset()
		f.m.Type = typeFilterRegister
		f.m.PutTypeInt(f.hooks)
		f.m.PutTypeInt(f.flags)
		if err = f.m.WriteTo(f.c); err != nil {
			return err
		}
	default:
		return fmt.Errorf("filter: unexpected imsg type=%s\n", filterTypeName[f.m.Type])
	}

	f.ready = true
	return nil
}

// Serve communicates with OpenSMTPD in a loop, until either one of the
// parties closes stdin.
func (f *Filter) Serve() error {
	var err error

	if !f.ready {
		if err = f.Register(); err != nil {
			return err
		}
	}

	if f.m == nil {
		f.m = new(message)
	}
	if f.session == nil {
		if f.session, err = lru.New(1024); err != nil {
			return err
		}
	}
	if f.c == nil {
		if f.c, err = newConn(0); err != nil {
			return err
		}
	}

	for {
		if err := f.m.ReadFrom(f.c); err != nil {
			if err.Error() != "resource temporarily unavailable" {
				return err
			}
		}
		if err := f.handle(); err != nil {
			return err
		}
	}
}

func (f *Filter) handle() (err error) {
	if t, ok := filterTypeName[f.m.Type]; ok {
		log.Printf("filter: imsg %s\n", t)
	} else {
		log.Printf("filter: imsg UNKNOWN %d\n", f.m.Type)
	}

	switch f.m.Type {
	case typeFilterEvent:
		if err = f.handleEvent(); err != nil {
			return
		}

	case typeFilterquery:
		if err = f.handlequery(); err != nil {
			return
		}
	}

	return
}

func fdCount() int {
	d, err := os.Open("/proc/self/fd")
	if err != nil {
		log.Printf("fdcount open: %v\n", err)
		return -1
	}
	defer d.Close()
	fds, err := d.Readdirnames(-1)
	if err != nil {
		log.Printf("fdcount: %v\n", err)
		return -1
	}
	return len(fds) - 1 // -1 for os.Open...
}

func (f *Filter) handleEvent() (err error) {
	var (
		id uint64
		t  int
	)

	if id, err = f.m.GetTypeID(); err != nil {
		return
	}
	if t, err = f.m.GetTypeInt(); err != nil {
		return
	}

	log.Printf("imsg event: %s [id=%#x]\n", eventName(t), id)
	log.Printf("imsg event data: %q\n", f.m.Data[14:])
	log.Printf("fdcount: %d [pid=%d]\n", fdCount(), os.Getpid())

	switch t {
	case eventConnect:
		f.session.Add(id, NewSession(f, id))
	case eventDisconnect:
		f.session.Remove(id)
	}

	return
}

func (f *Filter) handlequery() (err error) {
	var (
		id, qid uint64
		t       int
	)

	if id, err = f.m.GetTypeID(); err != nil {
		return
	}
	if qid, err = f.m.GetTypeID(); err != nil {
		return
	}
	if t, err = f.m.GetTypeInt(); err != nil {
		return
	}

	log.Printf("imsg query: %s [id=%#x,qid=%#x]\n", queryName(t), id, qid)
	//log.Printf("imsg query data (%d remaining): %q\n", len(f.m.Data[f.m.rpos:]), f.m.Data[f.m.rpos:])
	//log.Printf("fdcount: %d [pid=%d]\n", fdCount(), os.Getpid())

	var s *Session
	if cached, ok := f.session.Get(id); ok {
		s = cached.(*Session)
	} else {
		s = NewSession(f, id)
		f.session.Add(id, s)
	}
	s.qtype = t
	s.qid = qid

	switch t {
	case queryConnect:
		var query ConnectQuery
		if query.Local, err = f.m.GetTypeSockaddr(); err != nil {
			return
		}
		if query.Remote, err = f.m.GetTypeSockaddr(); err != nil {
			return
		}
		if query.Hostname, err = f.m.GetTypeString(); err != nil {
			return
		}

		log.Printf("query connect: %s\n", query)
		if f.Connect != nil {
			return f.Connect(s, &query)
		}

		log.Printf("filter: WARNING: no connect callback\n")

	case queryHELO:
		var line string
		if line, err = f.m.GetTypeString(); err != nil {
			return
		}

		log.Printf("query HELO: %q\n", line)
		if f.HELO != nil {
			return f.HELO(s, line)
		}

		log.Printf("filter: WARNING: no HELO callback\n")
		return f.respond(s, FilterOK, 0, "")

	case queryMAIL:
		var user, domain string
		if user, domain, err = f.m.GetTypeMailaddr(); err != nil {
			return
		}

		log.Printf("query MAIL: %s\n", user+"@"+domain)
		if f.MAIL != nil {
			return f.MAIL(s, user, domain)
		}

		log.Printf("filter: WARNING: no MAIL callback\n")
		return f.respond(s, FilterOK, 0, "")

	case queryRCPT:
		var user, domain string
		if user, domain, err = f.m.GetTypeMailaddr(); err != nil {
			return
		}

		log.Printf("query RCPT: %s\n", user+"@"+domain)
		if f.RCPT != nil {
			return f.RCPT(s, user, domain)
		}

		log.Printf("filter: WARNING: no RCPT callback\n")
		return f.respond(s, FilterOK, 0, "")

	case queryDATA:
		if f.DATA != nil {
			return f.DATA(s)
		}

		log.Printf("filter: WARNING: no DATA callback\n")
		return f.respond(s, FilterOK, 0, "")

	case queryEOM:
		var dataLen uint32
		if dataLen, err = f.m.GetTypeUint32(); err != nil {
			return
		}

		if f.EOM != nil {
			return f.EOM(s, dataLen)
		}

		log.Printf("filter: WARNING: no EOM callback\n")
		return f.respond(s, FilterOK, 0, "")
	}

	return
}

func (f *Filter) respond(s *Session, status, code int, line string) error {
	log.Printf("filter: %s %s [code=%d,line=%q]\n", filterName(typeFilterResponse), responseName(status), code, line)

	if s.qtype == queryEOM {
		// Not implemented
		return nil
	}

	m := new(message)
	m.Type = typeFilterResponse
	m.PutTypeID(s.qid)
	m.PutTypeInt(s.qtype)
	if s.qtype == queryEOM {
		// Not imlemented
		return nil
	}
	m.PutTypeInt(status)
	m.PutTypeInt(code)
	if line != "" {
		m.PutTypeString(line)
	}

	if err := m.WriteTo(f.c); err != nil {
		log.Printf("filter: respond failed: %v\n", err)
		return err
	}

	return nil
}

// ConnectQuery are the QUERY_CONNECT arguments
type ConnectQuery struct {
	Local, Remote net.Addr
	Hostname      string
}

func (q ConnectQuery) String() string {
	return fmt.Sprintf("%s -> %s [hostname=%s]", q.Remote, q.Local, q.Hostname)
}
