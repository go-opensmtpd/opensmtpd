package opensmtpd

import (
	"fmt"
	"net"
)

type ConnectQuery struct {
	Local, Remote net.Addr
	Hostname      string
}

func (q ConnectQuery) String() string {
	return fmt.Sprintf("%s -> %s [hostname=%s]", q.Remote, q.Local, q.Hostname)
}
