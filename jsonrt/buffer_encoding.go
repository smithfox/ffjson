package jsonrt

type EncodingBuffer interface {
	AppendByte(b byte)
	AppendBytes(s []byte)
	AppendString(s string)
	AppendJson(s []byte)
	AppendJsonString(s string)
	AppendInt(n int64, base int)
	AppendUint(u uint64, base int)
	AppendBool(t bool)
	AppendFloat(f float64, fmt byte, prec int, bitSize int)
	Encode(interface{}) error
	Bytes() []byte
	Grow(n int)
	Rewind(n int) error
}

func (buf *Buffer) AppendByte(b byte) {
	buf.WriteByte(b)
}

func (buf *Buffer) AppendBytes(s []byte) {
	buf.Write(s)
}

func (buf *Buffer) AppendString(s string) {
	buf.WriteString(s)
}

func (buf *Buffer) AppendInt(n int64, base int) {
	formatBits2(buf, uint64(n), base, n < 0)
}

func (buf *Buffer) AppendUint(u uint64, base int) {
	formatBits2(buf, u, base, false)
}

func (buf *Buffer) AppendFloat(f float64, fmt byte, prec int, bitSize int) {
	writeFloat(buf, f, fmt, prec, bitSize)
}

var bb_true []byte = []byte{'t', 'r', 'u', 'e'}
var bb_false []byte = []byte{'f', 'a', 'l', 's', 'e'}

func (buf *Buffer) AppendBool(t bool) {
	if t {
		buf.Write(bb_true)
	} else {
		buf.Write(bb_false)
	}
}

func (buf *Buffer) AppendJsonString(s string) {
	buf.AppendJson([]byte(s))
}

func (buf *Buffer) AppendJson(s []byte) {
	WriteJson(buf, s)
}

/*
func (buf *Buffer) Bytes() []byte {

}
*/
