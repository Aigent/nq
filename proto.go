package nq

import (
	"encoding/binary"
	"errors"
	"io"
)

// 	Binary protocol
// 	~~~~~~~~~~~~~~~
//
// +=======...=+===============... =+
// | size  ... |   payload     ...  |
// |UVarInt... | (size) octets ...  |
// +=======...=+===============... =+
// |           |                    |
// |<-[1..10]->|<-    (size)   ...->|
//
//  * size of the payload -- unsigned 64bit varint, [1..10] octets. Practically, size is limited by the MaxPayload value.
//  * payload: arbitrary bytes, [0 .. MaxPayload] bytes.

var (
	// MaxPayload prevents receiving side from DoS atack due to allocating too much RAM
	MaxPayload = 10 * 1024 * 1024 // 10Mb ought to be enough for anybody

	// ErrTooBig indicates that payload exceeds the limit
	ErrTooBig = errors.New("nanoq: payload exceeds the limit")
)

const maxHeaderLen = binary.MaxVarintLen64

type encoder struct {
	scratchpad [maxHeaderLen]byte
}

func (e *encoder) EncodeTo(w io.Writer, p []byte) error {
	size := len(p)
	if size > MaxPayload {
		return ErrTooBig
	}
	// write size header
	n := binary.PutUvarint(e.scratchpad[:], uint64(size))
	if _, err := w.Write(e.scratchpad[:n]); err != nil {
		return err
	}
	// write payload
	if _, err := w.Write(p); err != nil {
		return err
	}
	return nil
}

// decodeSize returns the size of a payload in bytes
func decodeSize(r io.ByteReader) (int, error) {
	// header: payload size
	size, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, err
	}
	if size > uint64(MaxPayload) {
		return 0, ErrTooBig
	}
	return int(size), nil
}
