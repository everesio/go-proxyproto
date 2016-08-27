// Package proxyproto implements Proxy Protocol (v1 and v2) parser and writer, as per specification:
// http://www.haproxy.org/download/1.5/doc/proxy-protocol.txt
package proxyproto

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
)

var (
	INET4 = "ip"
	INET6 = "ip6"

	// Protocol
	SIGV1 = []byte{'\x50', '\x52', '\x4F', '\x58', '\x59'}
	SIGV2 = []byte{'\x0D', '\x0A', '\x0D', '\x0A', '\x00', '\x0D', '\x0A', '\x51', '\x55', '\x49', '\x54', '\x0A'}

	ErrCantReadProtocolVersionAndCommand    = errors.New("Can't read proxy protocol version and command")
	ErrCantReadAddressFamilyAndProtocol     = errors.New("Can't read address family or protocol")
	ErrCantReadLength                       = errors.New("Can't read length")
	ErrCantResolveSourceUnixAddress         = errors.New("Can't resolve source Unix address")
	ErrCantResolveDestinationUnixAddress    = errors.New("Can't resolve destination Unix address")
	ErrNoProxyProtocol                      = errors.New("Proxy protocol signature not present")
	ErrUnknownProxyProtocolVersion          = errors.New("Unknown proxy protocol version")
	ErrUnsupportedProtocolVersionAndCommand = errors.New("Unsupported proxy protocol version and command")
	ErrUnsupportedAddressFamilyAndProtocol  = errors.New("Unsupported address family and protocol")
	ErrInvalidLength                        = errors.New("Invalid length")
	ErrInvalidAddress                       = errors.New("Invalid address")
	ErrInetFamilyDoesntMatchProtocol        = errors.New("IP address(es) family doesn't match protocol")
	ErrInvalidPortNumber                    = errors.New("Invalid port number")
)

// Header is the placeholder for proxy protocol header.
type Header struct {
	Version            byte
	Command            ProtocolVersionAndCommand
	TransportProtocol  AddressFamilyAndProtocol
	SourceAddress      net.IP
	DestinationAddress net.IP
	SourcePort         uint16
	DestinationPort    uint16
}

// EqualTo returns true if headers are equivalent, false otherwise.
func (header *Header) EqualTo(q *Header) bool {
	if header == nil || q == nil {
		return false
	}
	if header.Command.IsLocal() {
		return true
	}
	return header.TransportProtocol == q.TransportProtocol &&
		header.SourceAddress.String() == q.SourceAddress.String() &&
		header.DestinationAddress.String() == q.DestinationAddress.String() &&
		header.SourcePort == q.SourcePort &&
		header.DestinationPort == q.DestinationPort
}

// WriteTo renders a proxy protocol header in a format to write over the wire.
func (header *Header) WriteTo(w io.Writer) (int64, error) {
	switch header.Version {
	case 1:
		return header.writeVersion1(w)
	case 2:
		return header.writeVersion2(w)
	default:
		return 0, ErrUnknownProxyProtocolVersion
	}
}

// Read identifies the proxy protocol version and reads the remaining of
// the header, accordingly.
//
// If proxy protocol header signature is not present, the reader buffer remains untouched
// and is safe for reading outside of this code.
//
// If proxy protocol header signature is present but an error is raised while processing
// the remaining header, assume the reader buffer to be in a corrupt state.
func Read(reader *bufio.Reader) (*Header, error) {
	// Don't touch reader buffer before understanding if this is a valid header.
	signature, _ := reader.Peek(13)

	// Is it v1 or v2?
	if bytes.Equal(signature[:5], SIGV1) {
		return parseVersion1(reader)
	} else if bytes.Equal(signature[:12], SIGV2) {
		return parseVersion2(reader)
	}

	return nil, ErrNoProxyProtocol
}