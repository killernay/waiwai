package fec

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeSameSize(t *testing.T) {
	chunks := [][]byte{
		{0x01, 0x02, 0x03, 0x04},
		{0x10, 0x20, 0x30, 0x40},
		{0xAA, 0xBB, 0xCC, 0xDD},
		{0xFF, 0x00, 0xFF, 0x00},
	}

	parity := Encode(chunks)

	// ลอง recover แต่ละ chunk
	for i := 0; i < len(chunks); i++ {
		original := make([]byte, len(chunks[i]))
		copy(original, chunks[i])

		// ทำให้ chunk หาย
		missing := make([][]byte, len(chunks))
		for j := range chunks {
			if j == i {
				missing[j] = nil
			} else {
				missing[j] = chunks[j]
			}
		}

		recovered, err := Decode(missing, parity, i)
		if err != nil {
			t.Fatalf("chunk %d: decode error: %v", i, err)
		}
		if !bytes.Equal(recovered, original) {
			t.Fatalf("chunk %d: got %x, want %x", i, recovered, original)
		}
	}
}

func TestEncodeDecodeDifferentSize(t *testing.T) {
	chunks := [][]byte{
		{0x01, 0x02, 0x03},
		{0x10, 0x20, 0x30, 0x40, 0x50},
		{0xAA, 0xBB},
	}

	parity := Encode(chunks)

	// chunk 0 หาย
	missing := [][]byte{nil, chunks[1], chunks[2]}
	recovered, err := Decode(missing, parity, 0)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	// recovered ควรมีขนาดเท่า parity (max len)
	// 3 bytes แรกต้องตรงกับ original
	if !bytes.Equal(recovered[:3], chunks[0]) {
		t.Fatalf("first 3 bytes: got %x, want %x", recovered[:3], chunks[0])
	}
}

func TestTwoMissingFails(t *testing.T) {
	chunks := [][]byte{nil, nil, {0x01}}
	parity := []byte{0xFF}

	_, err := Decode(chunks, parity, 0)
	if err != ErrTooManyMissing {
		t.Fatalf("expected ErrTooManyMissing, got %v", err)
	}
}

func TestCanRecover(t *testing.T) {
	if CanRecover([][]byte{{1}, nil, {3}}, true) != true {
		t.Fatal("should be recoverable with 1 missing + parity")
	}
	if CanRecover([][]byte{nil, nil, {3}}, true) != false {
		t.Fatal("should not be recoverable with 2 missing")
	}
	if CanRecover([][]byte{{1}, nil, {3}}, false) != false {
		t.Fatal("should not be recoverable without parity")
	}
}

func BenchmarkEncode4MB(b *testing.B) {
	chunk := make([]byte, 4*1024*1024)
	for i := range chunk {
		chunk[i] = byte(i)
	}
	chunks := [][]byte{chunk, chunk, chunk, chunk}
	b.SetBytes(int64(len(chunk) * 4))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Encode(chunks)
	}
}
