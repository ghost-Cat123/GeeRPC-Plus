package GrowRPC

import (
	"io"
	"log"
)

type DebugWrapper struct {
	io.Reader
	io.Writer
	io.Closer
}

func (d *DebugWrapper) Read(p []byte) (n int, err error) {
	n, err = d.Reader.Read(p)
	log.Printf("Server Read %d bytes, err %v", n, err)
	return
}
