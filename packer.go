package main

import (
	"bytes"
	"compress/gzip"
	"io"
)

type Packer struct {
	buffer bytes.Buffer
}

func NewPacker(raw []byte) (*Packer, error) {
	p := &Packer{}

	unziped, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer unziped.Close()

	_, err = io.Copy(&p.buffer, unziped)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Packer) Flush() []byte {
	compresed := &bytes.Buffer{}
	gw := gzip.NewWriter(compresed)

	gw.Write(p.buffer.Bytes())
	gw.Close()

	p.buffer.Reset()
	return compresed.Bytes()
}
