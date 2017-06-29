package opensmtpd

import (
	"net"
	"os"
)

// NewConn wraps a file descriptor to a net.FileConn
func NewConn(fd int) (net.Conn, error) {
	f := os.NewFile(uintptr(fd), "")
	return net.FileConn(f)
}
