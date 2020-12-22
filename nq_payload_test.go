package nq_test

import (
	"encoding/binary"
	"hash/crc64"
	"math/rand"
	"sync"
	"testing"
	"time"
)

type testMsg []byte

var crcTable = crc64.MakeTable(crc64.ECMA)

const (
	timestampLen = 8 // int64
	checksumLen  = 8 // uint64
	minPayload   = 80
	maxPayload   = 250
)

func newTestMsg() *testMsg {
	length := minPayload + rand.Intn(maxPayload-minPayload)
	data := make([]byte, length+timestampLen+checksumLen)
	rand.Read(data[timestampLen+checksumLen:])
	binary.LittleEndian.PutUint64(data[timestampLen:], crc64.Checksum(data[timestampLen+checksumLen:], crcTable))
	m := testMsg(data)
	m.SetTimestamp()
	return &m
}

var testMsgPool = sync.Pool{New: func() interface{} { return newTestMsg() }}

func (m *testMsg) Age() time.Duration {
	ns := binary.LittleEndian.Uint64(m.Raw())
	return time.Since(time.Unix(0, int64(ns)))
}

func (m *testMsg) SetTimestamp() {
	binary.LittleEndian.PutUint64(m.Raw(), uint64(time.Now().UnixNano()))
}

func (m *testMsg) Checksum() uint64 {
	return binary.LittleEndian.Uint64(m.Raw()[timestampLen:])
}

func (m *testMsg) Ok() bool {
	data := m.Raw()[timestampLen+checksumLen:]
	return crc64.Checksum(data, crcTable) == m.Checksum()
}

func (m *testMsg) Raw() []byte { return []byte(*m) }

func TestTestMsgChecksum(t *testing.T) {
	p := testMsgPool.Get().(*testMsg)
	if !p.Ok() {
		t.Fail()
	}
	testMsgPool.Put(p)
}
