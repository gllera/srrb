package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

func jsonEncode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func printFormatted(format string, v any) error {
	var output []byte
	var err error
	switch format {
	case "yaml":
		output, err = yaml.Marshal(v)
	case "json":
		output, err = json.Marshal(v)
	}
	if err != nil {
		return fmt.Errorf("encoding %s: %w", format, err)
	}
	fmt.Printf("%s\n", output)
	return nil
}
