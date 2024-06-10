package main

import (
	"bytes"
	"encoding/json"
)

type JsonEncoder struct {
	buffer  bytes.Buffer
	encoder *json.Encoder
}

func New_JsonEncoder() *JsonEncoder {
	enc := &JsonEncoder{}
	enc.encoder = json.NewEncoder(&enc.buffer)
	enc.encoder.SetEscapeHTML(false)
	return enc
}

func (e *JsonEncoder) Encode(a any) ([]byte, error) {
	e.buffer.Reset()
	if err := e.encoder.Encode(a); err != nil {
		return nil, err
	}
	return e.buffer.Bytes(), nil
}
