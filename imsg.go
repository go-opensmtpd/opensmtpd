package opensmtpd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
)

const (
	ibufReadSize   = 65535
	imsgMaxSize    = 16384
	imsgHeaderSize = 4 + 2 + 2 + 4 + 4
	imsgVersion    = 14

	maxLocalPartSize  = (255 + 1)
	maxDomainPartSize = (255 + 1)
)

// MessageHeader is the header of an imsg frame (struct imsg_hdr)
type MessageHeader struct {
	Type   uint32
	Len    uint16
	Flags  uint16
	PeerID uint32
	PID    uint32
}

// message implements OpenBSD imsg
type Message struct {
	Header MessageHeader

	// Data is the Message payload.
	Data []byte

	// rpos is the read position in the current Data
	rpos int

	// buf is what we read from the socket (and remains)
	buf []byte
}

func (m *Message) reset() {
	m.Header.Type = 0
	m.Header.Len = 0
	m.Header.Flags = 0
	m.Header.PeerID = imsgVersion
	m.Header.PID = uint32(os.Getpid())
	m.Data = m.Data[:0]
	m.rpos = 0
	m.buf = m.buf[:0]
}

// ReadFrom reads a message from the specified net.Conn, parses the header and
// reads the data payload.
func (m *Message) ReadFrom(r io.Reader) error {
	m.reset()

	head := make([]byte, imsgHeaderSize)
	if _, err := r.Read(head); err != nil {
		return err
	}

	buf := bytes.NewBuffer(head)
	if err := binary.Read(buf, binary.LittleEndian, &m.Header); err != nil {
		return err
	}
	debugf("imsg header: %+v\n", m.Header)

	data := make([]byte, m.Header.Len-imsgHeaderSize)
	if _, err := r.Read(data); err != nil {
		return err
	}
	m.Data = data
	debugf("imsg data: %d / %q\n", len(m.Data), m.Data)

	return nil
}

// WriteTo marshals the message to wire format and sends it to the net.Conn
func (m *Message) WriteTo(w io.Writer) error {
	m.Header.Len = uint16(len(m.Data)) + imsgHeaderSize

	buf := new(bytes.Buffer)
	debugf("imsg header: %+v\n", m.Header)
	if err := binary.Write(buf, binary.LittleEndian, &m.Header); err != nil {
		return err
	}
	buf.Write(m.Data)
	debugf("imsg send: %d / %q\n", buf.Len(), buf.Bytes())

	_, err := w.Write(buf.Bytes())
	return err
}

func (m *Message) GetInt() (int, error) {
	if m.rpos+4 > len(m.Data) {
		return 0, io.ErrShortBuffer
	}
	i := binary.LittleEndian.Uint32(m.Data[m.rpos:])
	m.rpos += 4
	return int(i), nil
}

func (m *Message) GetUint32() (uint32, error) {
	if m.rpos+4 > len(m.Data) {
		return 0, io.ErrShortBuffer
	}
	u := binary.LittleEndian.Uint32(m.Data[m.rpos:])
	m.rpos += 4
	return u, nil
}

func (m *Message) GetSize() (uint64, error) {
	if m.rpos+8 > len(m.Data) {
		return 0, io.ErrShortBuffer
	}
	u := binary.LittleEndian.Uint64(m.Data[m.rpos:])
	m.rpos += 8
	return u, nil
}

func (m *Message) GetString() (string, error) {
	o := bytes.IndexByte(m.Data[m.rpos:], 0)
	if o < 0 {
		return "", errors.New("imsg: string not NULL-terminated")
	}

	s := string(m.Data[m.rpos : m.rpos+o])
	m.rpos += o
	return s, nil
}

func (m *Message) GetID() (uint64, error) {
	if m.rpos+8 > len(m.Data) {
		return 0, io.ErrShortBuffer
	}
	u := binary.LittleEndian.Uint64(m.Data[m.rpos:])
	m.rpos += 8
	return u, nil
}

// Sockaddr emulates the mess that is struct sockaddr
type Sockaddr []byte

func (sa Sockaddr) IP() net.IP {
	switch len(sa) {
	case 16: // IPv4, sockaddr_in
		return net.IP(sa[4:8])
	case 28: // IPv6, sockaddr_in6
		return net.IP(sa[8:24])
	default:
		return nil
	}
}

func (sa Sockaddr) Port() uint16 {
	switch len(sa) {
	case 16: // IPv4, sockaddr_in
		return binary.LittleEndian.Uint16(sa[2:4])
	case 28: // IPv6, sockaddr_in6
		return binary.LittleEndian.Uint16(sa[2:4])
	default:
		return 0
	}
}

func (sa Sockaddr) Network() string {
	return "bla"
}

func (sa Sockaddr) String() string {
	return fmt.Sprintf("%s:%d", sa.IP(), sa.Port())
}

func (m *Message) GetSockaddr() (net.Addr, error) {
	s, err := m.GetSize()
	if err != nil {
		return nil, err
	}
	if m.rpos+int(s) > len(m.Data) {
		return nil, io.ErrShortBuffer
	}

	a := make(Sockaddr, s)
	copy(a[:], m.Data[m.rpos:])
	m.rpos += int(s)

	return a, nil
}

func (m *Message) GetMailaddr() (user, domain string, err error) {
	var buf [maxLocalPartSize + maxDomainPartSize]byte
	if maxLocalPartSize+maxDomainPartSize > len(m.Data[m.rpos:]) {
		return "", "", io.ErrShortBuffer
	}
	copy(buf[:], m.Data[m.rpos:])
	m.rpos += maxLocalPartSize + maxDomainPartSize
	user = string(buf[:maxLocalPartSize])
	domain = string(buf[maxLocalPartSize:])
	return
}

func (m *Message) GetType(t uint8) error {
	if m.rpos >= len(m.Data) {
		return io.ErrShortBuffer
	}

	b := m.Data[m.rpos]
	m.rpos++
	if b != t {
		return mprocTypeErr{t, b}
	}
	return nil
}

func (m *Message) GetTypeInt() (int, error) {
	if err := m.GetType(mINT); err != nil {
		return 0, err
	}
	return m.GetInt()
}

func (m *Message) GetTypeUint32() (uint32, error) {
	if err := m.GetType(mUINT32); err != nil {
		return 0, err
	}
	return m.GetUint32()
}

func (m *Message) GetTypeSize() (uint64, error) {
	if err := m.GetType(mSIZET); err != nil {
		return 0, err
	}
	return m.GetSize()
}

func (m *Message) GetTypeString() (string, error) {
	if err := m.GetType(mSTRING); err != nil {
		return "", err
	}
	return m.GetString()
}

func (m *Message) GetTypeID() (uint64, error) {
	if err := m.GetType(mID); err != nil {
		return 0, err
	}
	return m.GetID()
}

func (m *Message) GetTypeSockaddr() (net.Addr, error) {
	if err := m.GetType(mSOCKADDR); err != nil {
		return nil, err
	}
	return m.GetSockaddr()
}

func (m *Message) GetTypeMailaddr() (user, domain string, err error) {
	if err = m.GetType(mMAILADDR); err != nil {
		return
	}
	return m.GetMailaddr()
}

func (m *Message) PutInt(v int) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	m.Data = append(m.Data, b[:]...)
	m.Header.Len += 4
}

func (m *Message) PutUint32(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	m.Data = append(m.Data, b[:]...)
	m.Header.Len += 4
}

func (m *Message) PutString(s string) {
	m.Data = append(m.Data, append([]byte(s), 0)...)
	m.Header.Len += uint16(len(s)) + 1
}

func (m *Message) PutID(id uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], id)
	m.Data = append(m.Data, b[:]...)
	m.Header.Len += 8
}

func (m *Message) PutType(t uint8) {
	m.Data = append(m.Data, t)
	m.Header.Len += 1
}

func (m *Message) PutTypeInt(v int) {
	m.PutType(mINT)
	m.PutInt(v)
}

func (m *Message) PutTypeUint32(v uint32) {
	m.PutType(mUINT32)
	m.PutUint32(v)
}

func (m *Message) PutTypeString(s string) {
	m.PutType(mSTRING)
	m.PutString(s)
}

func (m *Message) PutTypeID(id uint64) {
	m.PutType(mID)
	m.PutID(id)
}
