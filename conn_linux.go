//+build linux

package vsock

import (
	"net"
	"time"

	"golang.org/x/sys/unix"
)

var _ net.Conn = &conn{}

// A conn is the net.Conn implementation for VM sockets.
type conn struct {
	fd         connFD
	localAddr  *Addr
	remoteAddr *Addr
}

// Implement net.Conn for type conn.
func (c *conn) LocalAddr() net.Addr                { return c.localAddr }
func (c *conn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *conn) SetDeadline(t time.Time) error      { return c.fd.SetDeadline(t) }
func (c *conn) SetReadDeadline(t time.Time) error  { return c.fd.SetReadDeadline(t) }
func (c *conn) SetWriteDeadline(t time.Time) error { return c.fd.SetWriteDeadline(t) }
func (c *conn) Read(b []byte) (n int, err error)   { return c.fd.Read(b) }
func (c *conn) Write(b []byte) (n int, err error)  { return c.fd.Write(b) }
func (c *conn) Close() error                       { return c.fd.Close() }

// newConn creates a conn using an fd with the specified file name, local, and
// remote addresses.
func newConn(cfd connFD, file string, local, remote *Addr) (*conn, error) {
	return &conn{
		fd:         cfd,
		localAddr:  local,
		remoteAddr: remote,
	}, nil
}

// dialStream is the entry point for DialStream on Linux.
func dialStream(cid, port uint32) (net.Conn, error) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, err
	}

	rsa := &unix.SockaddrVM{
		CID:  cid,
		Port: port,
	}

	if err := unix.Connect(fd, rsa); err != nil {
		return nil, err
	}

	lsa, err := unix.Getsockname(fd)
	//lsa, err := cfd.Getsockname()
	if err != nil {
		return nil, err
	}

	cfd, err := newConnFD(fd)
	if err != nil {
		return nil, err
	}

	lsavm := lsa.(*unix.SockaddrVM)

	return dialStreamLinuxHandleError(cfd, cid, port, lsavm)
}

// dialStreamLinuxHandleError ensures that any errors from dialStreamLinux result
// in the socket being cleaned up properly.
func dialStreamLinuxHandleError(cfd connFD, cid, port uint32, lsa *unix.SockaddrVM) (net.Conn, error) {
	c, err := dialStreamLinux(cfd, cid, port, lsa)
	if err != nil {
		// If any system calls fail during setup, the socket must be closed
		// to avoid file descriptor leaks.
		_ = cfd.Close()
		return nil, err
	}

	return c, nil
}

// dialStreamLinux is the entry point for tests on Linux.
func dialStreamLinux(cfd connFD, cid, port uint32, lsa *unix.SockaddrVM) (net.Conn, error) {
	localAddr := &Addr{
		ContextID: lsa.CID,
		Port:      lsa.Port,
	}

	remoteAddr := &Addr{
		ContextID: cid,
		Port:      port,
	}

	// File name is the name of the local socket.
	return newConn(cfd, localAddr.fileName(), localAddr, remoteAddr)
}
