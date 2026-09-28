package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// DecodeSpec strictly decodes the resource spec into v. Unknown fields are
// rejected so the typed model cannot silently drift from the schema.
func DecodeSpec(res *Resource, v any) error {
	if len(res.Spec) == 0 || string(res.Spec) == "null" {
		return Errorf(res, "missing spec")
	}
	dec := json.NewDecoder(bytes.NewReader(res.Spec))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return Errorf(res, "invalid spec: %v", err)
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Errorf(res, "invalid spec: trailing data")
	}
	return nil
}

// Canonical returns the canonical JSON encoding of v, used to compare
// composed values independent of how they were authored.
func Canonical(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("canonical encoding: %v", err))
	}
	return string(b)
}
