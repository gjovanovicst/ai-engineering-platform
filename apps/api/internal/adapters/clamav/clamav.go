// Package clamav scans uploads through clamd's INSTREAM protocol.
package clamav

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// Scanner talks to clamd over TCP.
type Scanner struct {
	Addr    string
	Timeout time.Duration
}

// New returns a scanner for addr (host:port).
func New(addr string) *Scanner { return &Scanner{Addr: addr} }

// Scan streams the data to clamd and fails when the response is not OK.
func (s *Scanner) Scan(ctx context.Context, data []byte) error {
	if s == nil || s.Addr == "" {
		return nil
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("clamav: dial: %w", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return fmt.Errorf("clamav: command: %w", err)
	}
	for len(data) > 0 {
		n := len(data)
		if n > 32<<10 {
			n = 32 << 10
		}
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(n))
		if _, err := conn.Write(size[:]); err != nil {
			return fmt.Errorf("clamav: chunk size: %w", err)
		}
		if _, err := conn.Write(data[:n]); err != nil {
			return fmt.Errorf("clamav: chunk: %w", err)
		}
		data = data[n:]
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return fmt.Errorf("clamav: end: %w", err)
	}
	reply, err := io.ReadAll(io.LimitReader(conn, 4096))
	if err != nil {
		return fmt.Errorf("clamav: reply: %w", err)
	}
	out := strings.Trim(string(reply), "\x00\r\n ")
	if strings.HasSuffix(out, "OK") {
		return nil
	}
	return fmt.Errorf("clamav: %s", out)
}
