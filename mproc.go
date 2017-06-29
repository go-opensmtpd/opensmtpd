package opensmtpd

import (
	"fmt"
)

const (
	M_INT = iota
	M_UINT32
	M_SIZET
	M_TIME
	M_STRING
	M_DATA
	M_ID
	M_EVPID
	M_MSGID
	M_SOCKADDR
	M_MAILADDR
	M_ENVELOPE
)

var mprocTypeName = map[uint8]string{
	M_INT:      "M_INT",
	M_UINT32:   "M_UINT32",
	M_SIZET:    "M_SIZET",
	M_TIME:     "M_TIME",
	M_STRING:   "M_STRING",
	M_DATA:     "M_DATA",
	M_ID:       "M_ID",
	M_EVPID:    "M_EVPID",
	M_MSGID:    "M_MSGID",
	M_SOCKADDR: "M_SOCKADDR",
	M_MAILADDR: "M_MAILADDR",
	M_ENVELOPE: "M_ENVELOPE",
}

func mprocType(t uint8) string {
	if s, ok := mprocTypeName[t]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN %d", t)
}

type MProcTypeErr struct {
	want, got uint8
}

func (err MProcTypeErr) Error() string {
	return fmt.Sprintf("mproc: expected type %s, got %s",
		mprocType(err.want), mprocType(err.got))
}
