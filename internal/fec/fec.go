// Package fec implements XOR-based Forward Error Correction.
//
// สำหรับทุกๆ N data chunks จะสร้าง 1 parity chunk โดย XOR ข้อมูลทั้งหมดเข้าด้วยกัน
// ถ้า chunk ใด chunk หนึ่งในกลุ่มหาย สามารถกู้คืนได้จาก parity + chunks ที่เหลือ
//
// ตัวอย่าง: GroupSize=4 → ทุกๆ 4 data chunks สร้าง 1 parity chunk (overhead 25%)
//
// ข้อจำกัด: กู้ได้เฉพาะกรณีหายไม่เกิน 1 chunk ต่อกลุ่ม
// ถ้าต้องการกู้มากกว่านั้นต้องใช้ Reed-Solomon (อนาคต)
package fec

// Group represents a FEC group of data chunks + 1 parity chunk.
type Group struct {
	GroupID   int64    // group index (chunkIdx / GroupSize)
	GroupSize int      // number of data chunks in this group
	Data      [][]byte // data chunks (nil = missing)
	Parity    []byte   // XOR parity of all data chunks
}

// Encode สร้าง parity chunk จาก data chunks ทั้งหมดในกลุ่ม
func Encode(chunks [][]byte) []byte {
	if len(chunks) == 0 {
		return nil
	}

	// หา max length
	maxLen := 0
	for _, c := range chunks {
		if len(c) > maxLen {
			maxLen = len(c)
		}
	}

	parity := make([]byte, maxLen)
	for _, c := range chunks {
		xorInto(parity, c)
	}
	return parity
}

// Decode กู้คืน chunk ที่หายไปจาก parity + chunks ที่เหลือ
// missingIdx คือ index ใน chunks[] ที่หาย (chunks[missingIdx] == nil)
// คืน data ของ chunk ที่หาย หรือ error ถ้ากู้ไม่ได้
func Decode(chunks [][]byte, parity []byte, missingIdx int) ([]byte, error) {
	if missingIdx < 0 || missingIdx >= len(chunks) {
		return nil, ErrInvalidIndex
	}

	// นับ missing
	missingCount := 0
	for _, c := range chunks {
		if c == nil {
			missingCount++
		}
	}
	if missingCount == 0 {
		return chunks[missingIdx], nil // ไม่มีอะไรหาย
	}
	if missingCount > 1 {
		return nil, ErrTooManyMissing
	}

	// XOR parity กับ chunks ที่มีอยู่ → ได้ chunk ที่หาย
	recovered := make([]byte, len(parity))
	copy(recovered, parity)
	for i, c := range chunks {
		if i != missingIdx && c != nil {
			xorInto(recovered, c)
		}
	}
	return recovered, nil
}

// CanRecover ตรวจว่ากู้คืนได้ไหม (หายไม่เกิน 1 chunk และมี parity)
func CanRecover(chunks [][]byte, hasParity bool) bool {
	if !hasParity {
		return false
	}
	missingCount := 0
	for _, c := range chunks {
		if c == nil {
			missingCount++
		}
	}
	return missingCount <= 1
}

func xorInto(dst, src []byte) {
	n := len(dst)
	if len(src) < n {
		n = len(src)
	}
	// XOR ทีละ 8 bytes สำหรับความเร็ว
	i := 0
	for ; i+8 <= n; i += 8 {
		dst[i] ^= src[i]
		dst[i+1] ^= src[i+1]
		dst[i+2] ^= src[i+2]
		dst[i+3] ^= src[i+3]
		dst[i+4] ^= src[i+4]
		dst[i+5] ^= src[i+5]
		dst[i+6] ^= src[i+6]
		dst[i+7] ^= src[i+7]
	}
	for ; i < n; i++ {
		dst[i] ^= src[i]
	}
}
