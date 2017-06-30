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
	FilterVersion = 51
)

const (
	TypeFilterRegister uint32 = iota
	TypeFilterEvent
	TypeFilterQuery
	TypeFilterPipe
	TypeFilterResponse
)

var filterTypeName = map[uint32]string{
	TypeFilterRegister: "IMSG_FILTER_REGISTER",
	TypeFilterEvent:    "IMSG_FILTER_EVENT",
	TypeFilterQuery:    "IMSG_FILTER_QUERY",
	TypeFilterPipe:     "IMSG_FILTER_PIPE",
	TypeFilterResponse: "IMSG_FILTER_RESPONSE",
}

func filterName(t uint32) string {
	if s, ok := filterTypeName[t]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN %d", t)
}

const (
	HookConnect = 1 << iota
	HookHELO
	HookMAIL
	HookRCPT
	HookDATA
	HookEOM
	HookReset
	HookDisconnect
	HookCommit
	HookRollback
	HookDataLine
)

var hookTypeName = map[uint16]string{
	HookConnect:    "HOOK_CONNECT",
	HookHELO:       "HOOK_HELO",
	HookMAIL:       "HOOK_MAIL",
	HookRCPT:       "HOOK_RCPT",
	HookDATA:       "HOOK_DATA",
	HookEOM:        "HOOK_EOM",
	HookReset:      "HOOK_RESET",
	HookDisconnect: "HOOK_DISCONNECT",
	HookCommit:     "HOOK_COMMIT",
	HookRollback:   "HOOK_ROLLBACK",
	HookDataLine:   "HOOK_DATALINE",
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
	EventConnect = iota
	EventReset
	EventDisconnect
	EventTXBegin
	EventTXCommit
	EventTXRollback
)

var eventTypeName = map[int]string{
	EventConnect:    "EVENT_CONNECT",
	EventReset:      "EVENT_RESET",
	EventDisconnect: "EVENT_DISCONNECT",
	EventTXBegin:    "EVENT_TX_BEGIN",
	EventTXCommit:   "EVENT_TX_COMMIT",
	EventTXRollback: "EVENT_TX_ROLLBACK",
}

func eventName(t int) string {
	if s, ok := eventTypeName[t]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN %d", int(t))
}

const (
	QueryConnect = iota
	QueryHELO
	QueryMAIL
	QueryRCPT
	QueryDATA
	QueryEOM
	QueryDataLine
)

var queryTypeName = map[int]string{
	QueryConnect:  "QUERY_CONNECT",
	QueryHELO:     "QUERY_HELO",
	QueryMAIL:     "QUERY_MAIL",
	QueryRCPT:     "QUERY_RCPT",
	QueryDATA:     "QUERY_DATA",
	QueryEOM:      "QUERY_EOM",
	QueryDataLine: "QUERY_DATALINE",
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
	m *Message

	hooks   int
	flags   int
	ready   bool
	session *lru.Cache
}

func (f *Filter) OnConnect(fn func(*Session, *ConnectQuery) error) {
	f.Connect = fn
	f.hooks |= HookConnect
}

func (f *Filter) OnHELO(fn func(*Session, string) error) {
	f.HELO = fn
	f.hooks |= HookHELO
}

func (f *Filter) OnMAIL(fn func(*Session, string, string) error) {
	f.MAIL = fn
	f.hooks |= HookMAIL
}

func (f *Filter) OnRCPT(fn func(*Session, string, string) error) {
	f.RCPT = fn
	f.hooks |= HookRCPT
}

func (f *Filter) OnDATA(fn func(*Session) error) {
	f.DATA = fn
	f.hooks |= HookDATA
}

func (f *Filter) OnDataLine(fn func(*Session, string) error) {
	f.DataLine = fn
	f.hooks |= HookDataLine
}

// Register our filter with OpenSMTPD
func (f *Filter) Register() error {
	var err error
	if f.m == nil {
		f.m = new(Message)
	}
	if f.c == nil {
		if f.c, err = NewConn(0); err != nil {
			return err
		}
	}
	if err = f.m.ReadFrom(f.c); err != nil {
		return err
	}

	// Fill hooks mask
	if f.Connect != nil {
		f.hooks |= HookConnect
	}
	if f.HELO != nil {
		f.hooks |= HookHELO
	}
	if f.MAIL != nil {
		f.hooks |= HookMAIL
	}
	if f.RCPT != nil {
		f.hooks |= HookRCPT
	}
	if f.DATA != nil {
		f.hooks |= HookDATA
	}
	if f.DataLine != nil {
		f.hooks |= HookDataLine
	}
	if f.EOM != nil {
		f.hooks |= HookEOM
	}
	if f.Disconnect != nil {
		f.hooks |= HookDisconnect
	}
	if f.Commit != nil {
		f.hooks |= HookCommit
	}

	if t, ok := filterTypeName[f.m.Type]; ok {
		log.Printf("filter: imsg %s\n", t)
	} else {
		log.Printf("filter: imsg UNKNOWN %d\n", f.m.Type)
	}

	switch f.m.Type {
	case TypeFilterRegister:
		var err error
		if f.Version, err = f.m.GetTypeUint32(); err != nil {
			return err
		}
		if f.Name, err = f.m.GetTypeString(); err != nil {
			return err
		}
		log.Printf("register version=%d,name=%q\n", f.Version, f.Name)

		f.m.reset()
		f.m.Type = TypeFilterRegister
		f.m.PutTypeInt(f.hooks)
		f.m.PutTypeInt(f.flags)
		if err = f.m.WriteTo(f.c); err != nil {
			return err
		}
	default:
		return fmt.Errorf("filter: unexpected imsg type=%s\n", filterTypeName[f.m.Type])
	}

	return nil
}

// Serve communicates with OpenSMTPD in a loop, until either one of the
// parties closes stdin.
func (f *Filter) Serve() error {
	var err error
	if f.m == nil {
		f.m = new(Message)
	}
	if f.session == nil {
		if f.session, err = lru.New(1024); err != nil {
			return err
		}
	}
	if f.c == nil {
		if f.c, err = NewConn(0); err != nil {
			return err
		}
	}

	for {
		//log.Printf("fdcount: %d [pid=%d]\n", fdCount(), os.Getpid())
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
	case TypeFilterEvent:
		if err = f.handleEvent(); err != nil {
			return
		}

	case TypeFilterQuery:
		if err = f.handleQuery(); err != nil {
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
	case EventConnect:
		f.session.Add(id, NewSession(f, id))
	case EventDisconnect:
		f.session.Remove(id)
	}

	return
}

func (f *Filter) handleQuery() (err error) {
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
	case QueryConnect:
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

	case QueryHELO:
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

	case QueryMAIL:
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

	case QueryRCPT:
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

	case QueryDATA:
		if f.DATA != nil {
			return f.DATA(s)
		}

		log.Printf("filter: WARNING: no DATA callback\n")
		return f.respond(s, FilterOK, 0, "")

	case QueryEOM:
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
	log.Printf("filter: %s %s [code=%d,line=%q]\n", filterName(TypeFilterResponse), responseName(status), code, line)

	if s.qtype == QueryEOM {
		// Not implemented
		return nil
	}

	m := new(Message)
	m.Type = TypeFilterResponse
	m.PutTypeID(s.qid)
	m.PutTypeInt(s.qtype)
	if s.qtype == QueryEOM {
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
