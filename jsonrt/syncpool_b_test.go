package jsonrt

import (
	"bytes"
	"sync"
	"testing"
)

var pool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
var p = bytes.Repeat([]byte{'a'}, 100)

func BenchmarkReuse(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get().(*bytes.Buffer)
			buf.Write(p)
			_ = buf.String()
			buf.Reset()
			pool.Put(buf)
		}
	})
}
func BenchmarkNoReuse(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var buf bytes.Buffer
			buf.Write(p)
			_ = buf.String()
		}
	})
}
