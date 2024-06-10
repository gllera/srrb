package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

type Packer struct {
	buffer bytes.Buffer
}

func New_Packer(path string) *Packer {
	p := &Packer{}

	if _, err := os.Stat(path); err == nil {
		file, err := os.Open(path)
		if err != nil {
			fatal("Unable to open file.", "path", path, "err", err.Error())
		}
		defer file.Close()

		gReader, err := gzip.NewReader(file)
		if err != nil {
			fatal("Unable to read file.", "path", path, "err", err.Error())
		}
		defer gReader.Close()

		data, err := io.ReadAll(gReader)
		if err != nil {
			fatal("Unable to read file.", "path", path, "err", err.Error())
		}

		p.buffer.Write(data)
	} else if err := os.Mkdir(filepath.Dir(path), 0755); err != nil {
		fatal("Unable to make base tag path.", "path", filepath.Dir(path), "err", err.Error())
	}

	return p
}

func (p *Packer) flush(path string) {
	nf, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fatal("Unable to open file.", "path", path, "err", err.Error())
	}
	defer nf.Close()

	gw := gzip.NewWriter(nf)
	if _, err = gw.Write(p.buffer.Bytes()); err != nil {
		fatal("Unable to write file.", "path", path, "err", err.Error())
	}
	if err = gw.Close(); err != nil {
		fatal("Unable to close file.", "path", path, "err", err.Error())
	}

	p.buffer.Reset()
}
