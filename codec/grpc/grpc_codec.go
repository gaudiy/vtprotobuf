// Package grpc provides a gRPC [encoding.CodecV2] implementation using vtproto for serialization.
package grpc

import (
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/mem"

	// Guarantee that the built-in proto is called registered before this one
	// so that it can be replaced.
	_ "google.golang.org/grpc/encoding/proto"
)

// Name is the name registered for the proto compressor.
const Name = "proto"

type vtProtoMessage interface {
	MarshalToSizedBufferVT(data []byte) (int, error)
	UnmarshalVT([]byte) error
	SizeVT() int
}

var defaultBufferPool = mem.DefaultBufferPool()

// Codec is a [encoding.CodecV2] implementation which uses vtproto for marshaling and
// unmarshaling when possible, and falls back to the built-in proto codec.
type Codec struct {
	fallback encoding.CodecV2
}

var _ encoding.CodecV2 = (*Codec)(nil)

// Name implements [encoding.CodecV2].
func (Codec) Name() string { return Name }

func (c *Codec) Marshal(v any) (mem.BufferSlice, error) {
	if m, ok := v.(vtProtoMessage); ok {
		size := m.SizeVT()
		if mem.IsBelowBufferPoolingThreshold(size) {
			buf := make([]byte, size)
			if _, err := m.MarshalToSizedBufferVT(buf[:size]); err != nil {
				return nil, err
			}
			return mem.BufferSlice{mem.SliceBuffer(buf)}, nil
		}
		buf := defaultBufferPool.Get(size)
		if _, err := m.MarshalToSizedBufferVT((*buf)[:size]); err != nil {
			defaultBufferPool.Put(buf)
			return nil, err
		}
		return mem.BufferSlice{mem.NewBuffer(buf, defaultBufferPool)}, nil
	}

	return c.fallback.Marshal(v)
}

func (c *Codec) Unmarshal(data mem.BufferSlice, v any) error {
	if m, ok := v.(vtProtoMessage); ok {
		buf := data.MaterializeToBuffer(defaultBufferPool)
		defer buf.Free()
		return m.UnmarshalVT(buf.ReadOnlyData())
	}

	return c.fallback.Unmarshal(data, v)
}

func init() {
	encoding.RegisterCodecV2(&Codec{
		fallback: encoding.GetCodecV2("proto"),
	})
}
