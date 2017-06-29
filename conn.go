package opensmtpd

import (
	"net"
	"os"
)

func NewConn(fd int) (net.Conn, error) {
	f := os.NewFile(uintptr(fd), "")
	return net.FileConn(f)
}
