package opensmtpd

import (
	"net"
	"os"
)

// newConn wraps a file descriptor to a net.FileConn
func newConn(fd int) (net.Conn, error) {
	f := os.NewFile(uintptr(fd), "")
	return net.FileConn(f)
}
