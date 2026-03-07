// Package protocol defines the qt wire protocol v2.
//
// Stream layout:
//   Stream 0  — control (always open, bidirectional)
//   Stream 1+ — data streams (sender opens N, receiver accepts)
//
// Control messages are length-prefixed JSON for human-readability and
// forward-compatibility. Data chunks use a compact binary header.
package protocol

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

const (
	Version       = 2
	ALPN          = "waiwai/1"
	DefaultPort   = 4242
	ChunkSize     = 4 * 1024 * 1024  // 4 MB
	MaxStreams    = 32
	DefaultStreams = 8
)

// ─── Control message types ────────────────────────────────────────────────────

type MsgType string

const (
	MsgHello      MsgType = "hello"       // sender → receiver: session info
	MsgAccept     MsgType = "accept"      // receiver → sender: ok / resume offsets
	MsgFileInfo   MsgType = "file_info"   // sender → receiver: one per file
	MsgFileAck    MsgType = "file_ack"    // receiver → sender: ready / skip
	MsgStats      MsgType = "stats"       // bidirectional: periodic metrics
	MsgDone       MsgType = "done"        // sender → receiver: all files sent
	MsgError      MsgType = "error"       // either direction
)

// Envelope wraps every control message.
type Envelope struct {
	Type    MsgType         `json:"t"`
	Payload json.RawMessage `json:"p"`
}

func WriteMsg(w io.Writer, msgType MsgType, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	env, err := json.Marshal(Envelope{Type: msgType, Payload: raw})
	if err != nil {
		return err
	}
	// 4-byte length prefix
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(env)))
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	_, err = w.Write(env)
	return err
}

func ReadMsg(r io.Reader) (MsgType, json.RawMessage, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return "", nil, err
	}
	size := binary.BigEndian.Uint32(lenBuf[:])
	if size > 1<<20 { // 1 MB sanity cap on control messages
		return "", nil, fmt.Errorf("control message too large: %d", size)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", nil, err
	}
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "", nil, err
	}
	return env.Type, env.Payload, nil
}

// ─── Hello / Accept ───────────────────────────────────────────────────────────

type Hello struct {
	Version      int    `json:"version"`
	SessionID    string `json:"session_id"`   // UUID, used for resume
	FileCount    int    `json:"file_count"`
	TotalBytes   int64  `json:"total_bytes"`
	NumStreams   int    `json:"num_streams"`
	RateLimit    int64  `json:"rate_limit_bps"` // 0 = unlimited
	FECGroupSize int    `json:"fec_group_size"` // 0 = disabled, N = ทุก N chunks สร้าง 1 parity
}

type Accept struct {
	SessionID     string            `json:"session_id"`
	ResumeOffsets map[string]int64  `json:"resume_offsets"` // filename → bytes already received
}

// ─── FileInfo / FileAck ───────────────────────────────────────────────────────

type FileInfo struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`        // relative path, "/" separated
	Size       int64  `json:"size"`
	ModTime    int64  `json:"mod_time"`    // Unix seconds
	ChunkCount int64  `json:"chunk_count"`
	SHA256     string `json:"sha256"`      // whole-file hash (hex), optional
}

type FileAck struct {
	ID           int   `json:"id"`
	ResumeChunk  int64 `json:"resume_chunk"` // 0 = start fresh
	Skip         bool  `json:"skip"`         // receiver already has this file
}

// ─── Stats (monitoring) ───────────────────────────────────────────────────────

type Stats struct {
	Timestamp     int64   `json:"ts"`           // Unix ms
	BytesSent     int64   `json:"bytes_sent"`
	BytesAcked    int64   `json:"bytes_acked"`
	Throughput    float64 `json:"throughput_mbps"`
	RTTms         float64 `json:"rtt_ms"`
	PacketLoss    float64 `json:"packet_loss_pct"`
	ActiveStreams  int     `json:"active_streams"`
	FilesComplete int     `json:"files_complete"`
}

// ─── Error ────────────────────────────────────────────────────────────────────

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ─── Binary chunk header (compact, on data streams) ──────────────────────────
// Layout: [fileID:2][chunkIdx:8][flags:1][dataLen:4][checksum:32] = 47 bytes
//
// flags:
//   bit 0 = FEC parity chunk (1 = parity, 0 = data)

const ChunkHeaderSize = 2 + 8 + 1 + 4 + 32

const (
	FlagData   byte = 0x00
	FlagParity byte = 0x01
)

type ChunkHeader struct {
	FileID   uint16
	ChunkIdx int64
	Flags    byte
	DataLen  uint32
	Checksum [32]byte
}

func WriteChunk(w io.Writer, fileID uint16, chunkIdx int64, data []byte) error {
	return WriteChunkFlags(w, fileID, chunkIdx, FlagData, data)
}

func WriteChunkFlags(w io.Writer, fileID uint16, chunkIdx int64, flags byte, data []byte) error {
	h := ChunkHeader{
		FileID:   fileID,
		ChunkIdx: chunkIdx,
		Flags:    flags,
		DataLen:  uint32(len(data)),
		Checksum: sha256.Sum256(data),
	}
	buf := make([]byte, ChunkHeaderSize)
	binary.BigEndian.PutUint16(buf[0:], h.FileID)
	binary.BigEndian.PutUint64(buf[2:], uint64(h.ChunkIdx))
	buf[10] = h.Flags
	binary.BigEndian.PutUint32(buf[11:], h.DataLen)
	copy(buf[15:], h.Checksum[:])
	if _, err := w.Write(buf); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func ReadChunk(r io.Reader) (fileID uint16, chunkIdx int64, data []byte, err error) {
	var flags byte
	fileID, chunkIdx, flags, data, err = ReadChunkFlags(r)
	_ = flags
	return
}

func ReadChunkFlags(r io.Reader) (fileID uint16, chunkIdx int64, flags byte, data []byte, err error) {
	buf := make([]byte, ChunkHeaderSize)
	if _, err = io.ReadFull(r, buf); err != nil {
		return
	}
	h := ChunkHeader{}
	h.FileID = binary.BigEndian.Uint16(buf[0:])
	h.ChunkIdx = int64(binary.BigEndian.Uint64(buf[2:]))
	h.Flags = buf[10]
	h.DataLen = binary.BigEndian.Uint32(buf[11:])
	copy(h.Checksum[:], buf[15:])

	if h.DataLen > ChunkSize*2 {
		err = fmt.Errorf("chunk data len suspiciously large: %d", h.DataLen)
		return
	}
	data = make([]byte, h.DataLen)
	if _, err = io.ReadFull(r, data); err != nil {
		return
	}
	got := sha256.Sum256(data)
	if got != h.Checksum {
		err = fmt.Errorf("file %d chunk %d: checksum mismatch", h.FileID, h.ChunkIdx)
		return
	}
	return h.FileID, h.ChunkIdx, h.Flags, data, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func NowMS() int64 { return time.Now().UnixMilli() }
