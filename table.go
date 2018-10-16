package opensmtpd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

const (
	procTableOK = iota
	procTableFail
	procTableOpen
	procTableClose
	procTableUpdate
	procTableCheck
	procTableLookup
	procTableFetch
)

var procTableTypeName = map[uint32]string{
	procTableOK:     "PROC_TABLE_OK",
	procTableFail:   "PROC_TABLE_FAIL",
	procTableOpen:   "PROC_TABLE_OPEN",
	procTableClose:  "PROC_TABLE_CLOSE",
	procTableUpdate: "PROC_TABLE_UPDATE",
	procTableCheck:  "PROC_TABLE_CHECK",
	procTableLookup: "PROC_TABLE_LOOKUP",
	procTableFetch:  "PROC_TABLE_FETCH",
}

func procTableName(t uint32) string {
	if s, ok := procTableTypeName[t]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN %d", t)
}

// Table implements the OpenSMTPD table API
type Table struct {
	// Update callback
	Update func() (int, error)

	// Check callback
	Check func(service int, params Dict, key string) (int, error)

	// Lookup callback
	Lookup func(service int, params Dict, key string) (string, error)

	// Fetch callback
	Fetch func(service int, params Dict) (string, error)

	// Close callback, called at stop
	Close func() error

	c      net.Conn
	m      *message
	closed bool
}

func (t *Table) Serve() error {
	var err error

	if t.c, err = newConn(0); err != nil {
		return err
	}

	t.m = new(message)

	for !t.closed {
		if err = t.m.ReadFrom(t.c); err != nil {
			if err.Error() != "resource temporarily unavailable" {
				return fmt.Errorf("read error: %v", err)
			}
		}
		debugf("table: %s", procTableName(t.m.Header.Type))
		if err = t.dispatch(); err != nil {
			return fmt.Errorf("dispatch error: %v", err)
		}
	}

	return err
}

type tableOpenParams struct {
	Version uint32
	Name    [maxLineSize]byte
}

func (t *Table) dispatch() (err error) {
	switch t.m.Header.Type {
	case procTableOpen:
		/*
			var op tableOpenParams
			if err = t.getMessage(&op, maxLineSize+4); err != nil {
				return
			}

			if op.Version != TableVersion {
				fatalf("table: bad API version %d (we support %d)", op.Version, TableVersion)
			}
			if bytes.IndexByte(op.Name[:], 0) <= 0 {
				fatal("table: no name supplied")
			}
		*/
		var version uint32
		if version, err = t.m.GetUint32(); err != nil {
			return
		} else if version != TableVersion {
			fatalf("table: expected API version %d, got %d", TableVersion, version)
		}

		var name string
		if name, err = t.m.GetString(); err != nil {
			return
		} else if name == "" {
			fatal("table: no name supplied by smtpd!?")
		}

		debugf("table: version=%d name=%q\n", version, name)

		m := new(message)
		m.Header.Type = procTableOK
		m.Header.Len = imsgHeaderSize
		m.Header.PID = uint32(os.Getpid())
		if err = m.WriteTo(t.c); err != nil {
			return
		}

	case procTableUpdate:
		var r = 1

		if t.Update != nil {
			if r, err = t.Update(); err != nil {
				return
			}
		}

		m := new(message)
		m.Header.Type = procTableOK
		m.PutInt(r)
		if err = m.WriteTo(t.c); err != nil {
			return
		}

	case procTableClose:
		if t.Close != nil {
			if err = t.Close(); err != nil {
				return
			}
		}

		t.closed = true
		return

	case procTableCheck:
		var service int
		if service, err = t.m.GetInt(); err != nil {
			return
		}

		var params Dict
		if params, err = t.getParams(); err != nil {
			return
		}

		var key string
		if key, err = t.m.GetString(); err != nil {
			return
		}

		debugf("table_check: service=%s,params=%+v,key=%q",
			serviceName(service), params, key)

		var r = -1
		if t.Check != nil {
			if r, err = t.Check(service, params, key); err != nil {
				return
			}
		}

		log.Printf("table_check: result=%d\n", r)

		m := new(message)
		m.Header.Type = procTableOK
		m.Header.PID = uint32(os.Getpid())
		m.PutInt(r)
		if err = m.WriteTo(t.c); err != nil {
			return
		}

	case procTableLookup:
		var service int
		if service, err = t.m.GetInt(); err != nil {
			return
		}

		var params Dict
		if params, err = t.getParams(); err != nil {
			return
		}

		var key string
		if key, err = t.m.GetString(); err != nil {
			return
		}

		debugf("table_lookup: service=%s,params=%+v,key=%q",
			serviceName(service), params, key)

		var val string
		if t.Lookup != nil {
			if val, err = t.Lookup(service, params, key); err != nil {
				return
			}
		}

		m := new(message)
		m.Header.Type = procTableOK
		m.Header.PID = uint32(os.Getpid())
		if val == "" {
			m.PutInt(-1)
		} else {
			m.PutInt(1)
			m.PutString(val)
		}
		if err = m.WriteTo(t.c); err != nil {
			return
		}

	case procTableFetch:
		var service int
		if service, err = t.m.GetInt(); err != nil {
			return
		}

		var params Dict
		if params, err = t.getParams(); err != nil {
			return
		}

		debugf("table_fetch: service=%s,params=%+v",
			serviceName(service), params)

		var val string
		if t.Fetch != nil {
			if val, err = t.Fetch(service, params); err != nil {
				return
			}
		}

		m := new(message)
		m.Header.Type = procTableOK
		m.Header.PID = uint32(os.Getpid())
		if val == "" {
			m.PutInt(-1)
		} else {
			m.PutInt(1)
			m.PutString(val)
		}
		if err = m.WriteTo(t.c); err != nil {
			return
		}

	}

	return nil
}

func (t *Table) getMessage(data interface{}, size int) (err error) {
	buf := make([]byte, size)
	if _, err = io.ReadFull(t.c, buf); err != nil {
		return
	}

	return binary.Read(bytes.NewBuffer(buf), binary.LittleEndian, data)
}

func (t *Table) getParams() (params Dict, err error) {
	var count uint64
	if count, err = t.m.GetSize(); err != nil {
		return
	}
	debugf("params: %d pairs", count)

	params = make(Dict, count)
	if count == 0 {
		return
	}

	var k, v string
	for ; count != 0; count-- {
		if k, err = t.m.GetString(); err != nil {
			return
		}
		if v, err = t.m.GetString(); err != nil {
			return
		}
		params[k] = v
	}

	return
}
