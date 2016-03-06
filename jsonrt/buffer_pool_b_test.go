package jsonrt

import (
	"bytes"
	"testing"
)

var p1 = bytes.Repeat([]byte{'a'}, 70)

func BenchmarkUseBufferPool(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := NewBuffer(nil)
			buf.Write(p1)
			_ = buf.String()
		}
	})
}

func BenchmarkNoUseBufferPool(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bb := make([]byte, 72)
			buf := bytes.NewBuffer(bb)
			buf.Write(p1)
			_ = buf.String()
		}
	})
}
