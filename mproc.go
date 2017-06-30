package opensmtpd

import (
	"fmt"
)

const (
	mINT = iota
	mUINT32
	mSIZET
	mTIME
	mSTRING
	mDATA
	mID
	mEVPID
	mMSGID
	mSOCKADDR
	mMAILADDR
	mENVELOPE
)

var mprocTypeName = map[uint8]string{
	mINT:      "M_INT",
	mUINT32:   "M_UINT32",
	mSIZET:    "M_SIZET",
	mTIME:     "M_TIME",
	mSTRING:   "M_STRING",
	mDATA:     "M_DATA",
	mID:       "M_ID",
	mEVPID:    "M_EVPID",
	mMSGID:    "M_MSGID",
	mSOCKADDR: "M_SOCKADDR",
	mMAILADDR: "M_MAILADDR",
	mENVELOPE: "M_ENVELOPE",
}

func mprocType(t uint8) string {
	if s, ok := mprocTypeName[t]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN %d", t)
}

type mprocTypeErr struct {
	want, got uint8
}

func (err mprocTypeErr) Error() string {
	return fmt.Sprintf("mproc: expected type %s, got %s",
		mprocType(err.want), mprocType(err.got))
}
