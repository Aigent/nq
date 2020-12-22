package nq

import (
	"fmt"
	"io"
)

// ringBuf is a ReadWriter with underlying memory arena
// when write position reaches the buffer capacity, it overflows and starts from 0
type ringBuf struct {
	buf  []byte
	r, w int

	hasData bool
}

// used in grow
const minSize = 256

// read the previously written bytes in two parts to account for a write wraparound
func (b *ringBuf) read() ([]byte, []byte) {
	if !b.hasData {
		return nil, nil
	}

	if b.w > b.r {
		// data is linear
		return b.buf[b.r:b.w], nil
	}

	// data wraps around zero
	return b.buf[b.r:], b.buf[:b.w]
}

// advanceRead moves the read pointer
func (b *ringBuf) advanceRead(n int) {
	if n > b.Len() || n < 0 {
		panic(fmt.Sprintf("nanoq.RingBuffer: attempt to read %d bytes from %d available", n, b.Len()))
	}
	if n == 0 {
		// return to avoid possible division by zero in the next line
		return
	}
	b.r = (b.r + n) % cap(b.buf)
	if b.r == b.w {
		b.hasData = false
	}
}

// Read the next len(p) bytes from the buffer or until no data is left.
// It returns the number of bytes read or io.EOF error if no data is available.
func (b *ringBuf) Read(p []byte) (n int, err error) {
	p1, p2 := b.read()
	if len(p1)+len(p2) == 0 {
		// empty
		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}

	n = copy(p, p1)
	n += copy(p[n:], p2)
	b.advanceRead(n)
	return n, nil
}

// ReadByte makes ringBuf io.ByteReader compatible.
// It returns io.EOF error if there is no byte available for reading.
func (b *ringBuf) ReadByte() (byte, error) {
	p1, p2 := b.read()
	if len(p1)+len(p2) == 0 {
		return 0, io.EOF
	}
	var bt byte
	if len(p1) > 0 {
		bt = p1[0]
	} else {
		bt = p2[0]
	}
	b.advanceRead(1)
	return bt, nil
}

// Write p into the memory buffer and return the number of bytes written.
// The buffer will grow automatically.
func (b *ringBuf) Write(p []byte) (int, error) {
	// ensure we have enough space
	if b.Cap()-b.Len() < len(p) {
		b.grow(len(p))
	}

	var n int

	if b.w < b.r {
		// the unread portion is linear
		n = copy(b.buf[b.w:], p)
	} else {
		// the unread portion wraps around
		n = copy(b.buf[b.w:], p)
		n += copy(b.buf, p[n:])
	}
	b.w = (b.w + n) % cap(b.buf)
	b.hasData = true
	return n, nil
}

// WriteTo writes data to w until the buffer is drained or an error occurs.
func (b *ringBuf) WriteTo(w io.Writer) (n int64, err error) {
	p1, p2 := b.read()
	l := len(p1) + len(p2)
	if l == 0 {
		return 0, nil
	}
	var nBytes int64
	for _, p := range [][]byte{p1, p2} {
		n, err := w.Write(p)
		b.advanceRead(n)
		nBytes += int64(n)
		if err != nil {
			return nBytes, err
		}
		if n != len(p) {
			return nBytes, io.ErrShortWrite
		}
	}

	return nBytes, nil
}

func (b *ringBuf) grow(n int) {
	p1, p2 := b.read()
	newCap := len(p1) + len(p2) + n
	if newCap < minSize {
		newCap = minSize
	}
	if newCap < b.Cap()*2 {
		newCap = b.Cap() * 2
	}

	newBuf := make([]byte, newCap)
	copy(newBuf, p1)
	copy(newBuf[len(p1):], p2)
	b.buf = newBuf
	b.r = 0
	b.w = len(p1) + len(p2)
}

func (b *ringBuf) Cap() int {
	return cap(b.buf)
}

func (b *ringBuf) Len() int {
	p1, p2 := b.read()
	return len(p1) + len(p2)
}

func (b *ringBuf) Reset() {
	b.hasData = false
	b.r = 0
	b.w = 0
}

func (b *ringBuf) Discard(n int) error {
	if n < 0 {
		panic("nanoq.*ringBuf.Discard: negative count")
	}
	p1, p2 := b.read()
	if len(p1)+len(p2) < n {
		return io.ErrUnexpectedEOF
	}
	b.advanceRead(n)
	return nil
}
