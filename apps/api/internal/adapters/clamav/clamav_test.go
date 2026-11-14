package clamav

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
)

func TestScanStreamsAndAccepts(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 10)
		_, _ = io.ReadFull(conn, buf) // zINSTREAM\x00
		for {
			var size [4]byte
			if _, err := io.ReadFull(conn, size[:]); err != nil {
				return
			}
			n := binary.BigEndian.Uint32(size[:])
			if n == 0 {
				_, _ = conn.Write([]byte("stream: OK\x00"))
				return
			}
			_, _ = io.CopyN(io.Discard, conn, int64(n))
		}
	}()
	if err := New(ln.Addr().String()).Scan(context.Background(), []byte("hello")); err != nil {
		t.Fatal(err)
	}
}
