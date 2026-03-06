// Package transfer implements the qt sender and receiver engines.
package transfer

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/waiwai-transfer/waiwai/internal/checkpoint"
	"github.com/waiwai-transfer/waiwai/internal/monitor"
	"github.com/waiwai-transfer/waiwai/internal/throttle"
	"github.com/waiwai-transfer/waiwai/internal/ui"
	"github.com/waiwai-transfer/waiwai/pkg/protocol"
)

// ─── SendOptions ─────────────────────────────────────────────────────────────

type SendOptions struct {
	Addr        string
	Paths       []string // files or directories
	NumStreams   int
	RateLimitMB int64  // MB/s, 0 = unlimited
	SessionID   string // for resume; auto-generated if empty
	MonitorAddr string // e.g. ":9090"
	TLSConfig   *tls.Config
}

// Send connects to a receiver and transfers all files.
func Send(ctx context.Context, opts SendOptions) error {
	if opts.NumStreams == 0 {
		opts.NumStreams = protocol.DefaultStreams
	}
	if opts.SessionID == "" {
		opts.SessionID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// Enumerate all files
	files, err := collectFiles(opts.Paths)
	if err != nil {
		return fmt.Errorf("collect files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no files to send")
	}

	// Compute total size
	var totalBytes int64
	for _, f := range files {
		totalBytes += f.Size
	}

	// Monitoring
	monitor.Global.FilesTotal.Store(int32(len(files)))
	if opts.MonitorAddr != "" {
		monitor.Serve(opts.MonitorAddr, totalBytes)
		fmt.Printf("📊 Metrics: http://%s/status\n", opts.MonitorAddr)
	}

	// Rate limiter (shared across all streams)
	limiter := throttle.New(opts.RateLimitMB * 1024 * 1024)

	// Connect
	tlsCfg := opts.TLSConfig
	if tlsCfg == nil {
		tlsCfg = clientTLS()
	}
	conn, err := quic.DialAddr(ctx, opts.Addr, tlsCfg, quicConfig())
	if err != nil {
		return fmt.Errorf("connect to %s: %w", opts.Addr, err)
	}
	defer conn.CloseWithError(0, "done")

	// Control stream
	ctrl, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return err
	}

	// Handshake
	if err := protocol.WriteMsg(ctrl, protocol.MsgHello, protocol.Hello{
		Version:    protocol.Version,
		SessionID:  opts.SessionID,
		FileCount:  len(files),
		TotalBytes: totalBytes,
		NumStreams:  opts.NumStreams,
		RateLimit:  opts.RateLimitMB * 1024 * 1024,
	}); err != nil {
		return err
	}

	_, raw, err := protocol.ReadMsg(ctrl)
	if err != nil {
		return err
	}
	var accept protocol.Accept
	if err := json.Unmarshal(raw, &accept); err != nil {
		return err
	}

	// UI
	display := ui.New(nil)
	display.Start()
	defer display.Stop()

	// Send each file
	for i, fi := range files {
		resumeChunk := int64(0)
		if offset, ok := accept.ResumeOffsets[fi.Name]; ok {
			resumeChunk = offset / protocol.ChunkSize
		}

		// FileInfo handshake
		chunkCount := (fi.Size + protocol.ChunkSize - 1) / protocol.ChunkSize
		if err := protocol.WriteMsg(ctrl, protocol.MsgFileInfo, protocol.FileInfo{
			ID:         i,
			Name:       fi.Name,
			Size:       fi.Size,
			ModTime:    fi.ModTime,
			ChunkCount: chunkCount,
		}); err != nil {
			return err
		}

		_, raw, err := protocol.ReadMsg(ctrl)
		if err != nil {
			return err
		}
		var ack protocol.FileAck
		if err := json.Unmarshal(raw, &ack); err != nil {
			return err
		}
		if ack.Skip {
			monitor.Global.FilesComplete.Add(1)
			continue
		}
		resumeChunk = ack.ResumeChunk

		fp := display.AddFile(fi.Name, fi.Size-resumeChunk*protocol.ChunkSize)
		if err := sendFile(ctx, conn, fi, uint16(i), resumeChunk, chunkCount,
			opts.NumStreams, limiter, fp); err != nil {
			monitor.Global.Errors.Add(1)
			return fmt.Errorf("send %s: %w", fi.Name, err)
		}
		monitor.Global.FilesComplete.Add(1)
	}

	protocol.WriteMsg(ctrl, protocol.MsgDone, struct{}{})
	return nil
}

func sendFile(
	ctx context.Context,
	conn quic.Connection,
	fi fileEntry,
	fileID uint16,
	resumeChunk, chunkCount int64,
	numStreams int,
	limiter *throttle.Limiter,
	fp *ui.FileProgress,
) error {
	f, err := os.Open(fi.AbsPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Build chunk list (skipping already sent chunks)
	var chunks []int64
	for i := resumeChunk; i < chunkCount; i++ {
		chunks = append(chunks, i)
	}

	// Distribute chunks across streams
	chunkCh := make(chan int64, len(chunks))
	for _, c := range chunks {
		chunkCh <- c
	}
	close(chunkCh)

	var wg sync.WaitGroup
	errCh := make(chan error, numStreams)
	actual := numStreams
	if int64(actual) > int64(len(chunks)) {
		actual = len(chunks)
	}

	monitor.Global.ActiveStreams.Add(int32(actual))
	defer monitor.Global.ActiveStreams.Add(-int32(actual))

	for s := 0; s < actual; s++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stream, err := conn.OpenStreamSync(ctx)
			if err != nil {
				errCh <- err
				return
			}
			defer stream.Close()

			buf := make([]byte, protocol.ChunkSize)
			for chunkIdx := range chunkCh {
				offset := chunkIdx * protocol.ChunkSize
				n, err := f.ReadAt(buf, offset)
				if err != nil && err != io.EOF {
					errCh <- fmt.Errorf("read chunk %d: %w", chunkIdx, err)
					return
				}
				data := buf[:n]

				var w io.Writer = stream
				if limiter != nil {
					w = throttle.NewWriter(stream, limiter)
				}
				if err := protocol.WriteChunk(w, fileID, chunkIdx, data); err != nil {
					errCh <- fmt.Errorf("write chunk %d: %w", chunkIdx, err)
					return
				}
				fp.Add(int64(n))
				monitor.Global.BytesSent.Add(int64(n))
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// ─── File collection ──────────────────────────────────────────────────────────

type fileEntry struct {
	Name    string // relative path for wire protocol
	AbsPath string
	Size    int64
	ModTime int64
}

func collectFiles(paths []string) ([]fileEntry, error) {
	var out []fileEntry
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			err = filepath.Walk(p, func(path string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(filepath.Dir(p), path)
				out = append(out, fileEntry{
					Name:    filepath.ToSlash(rel),
					AbsPath: path,
					Size:    fi.Size(),
					ModTime: fi.ModTime().Unix(),
				})
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			out = append(out, fileEntry{
				Name:    info.Name(),
				AbsPath: p,
				Size:    info.Size(),
				ModTime: info.ModTime().Unix(),
			})
		}
	}
	return out, nil
}

// ─── QUIC helpers ─────────────────────────────────────────────────────────────

func quicConfig() *quic.Config {
	return &quic.Config{
		MaxIdleTimeout:        60 * time.Second,
		MaxIncomingStreams:     512,
		MaxIncomingUniStreams:  -1,
		EnableDatagrams:       false,
	}
}

func clientTLS() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"waiwai/1"},
	}
}

// ─── ReceiveOptions ───────────────────────────────────────────────────────────

type ReceiveOptions struct {
	ListenAddr  string
	OutDir      string
	MonitorAddr string
	TLSConfig   *tls.Config
}

// Receive listens and saves incoming files.
func Receive(ctx context.Context, opts ReceiveOptions) error {
	tlsCfg := opts.TLSConfig
	if tlsCfg == nil {
		var err error
		tlsCfg, err = GenerateSelfSignedTLS()
		if err != nil {
			return err
		}
	}

	ln, err := quic.ListenAddr(opts.ListenAddr, tlsCfg, quicConfig())
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()
	fmt.Printf("📥 Listening on %s\n", opts.ListenAddr)

	if opts.MonitorAddr != "" {
		monitor.Serve(opts.MonitorAddr, 0)
		fmt.Printf("📊 Metrics: http://%s/status\n", opts.MonitorAddr)
	}

	for {
		conn, err := ln.Accept(ctx)
		if err != nil {
			return err
		}
		go func() {
			if err := handleConn(ctx, conn, opts.OutDir); err != nil {
				fmt.Fprintf(os.Stderr, "connection error: %v\n", err)
			}
		}()
	}
}

func handleConn(ctx context.Context, conn quic.Connection, outDir string) error {
	defer conn.CloseWithError(0, "done")

	ctrl, err := conn.AcceptStream(ctx)
	if err != nil {
		return err
	}

	// Read Hello
	_, raw, err := protocol.ReadMsg(ctrl)
	if err != nil {
		return err
	}
	var hello protocol.Hello
	if err := json.Unmarshal(raw, &hello); err != nil {
		return err
	}

	sessionDir := filepath.Join(outDir)
	os.MkdirAll(sessionDir, 0755)

	// Load resume state for all files in this session
	resumeOffsets := map[string]int64{}

	if err := protocol.WriteMsg(ctrl, protocol.MsgAccept, protocol.Accept{
		SessionID:     hello.SessionID,
		ResumeOffsets: resumeOffsets,
	}); err != nil {
		return err
	}

	// Display
	display := ui.New(nil)
	display.Start()
	defer display.Stop()

	// Map fileID → state for parallel stream handling
	type fileState struct {
		info       protocol.FileInfo
		outFile    *os.File
		checkpoint *checkpoint.State
		progress   *ui.FileProgress
		mu         sync.Mutex
	}
	fileMap := sync.Map{}

	// Accept data streams in background
	streamErr := make(chan error, 128)
	go func() {
		for {
			stream, err := conn.AcceptStream(ctx)
			if err != nil {
				return
			}
			go func(s quic.Stream) {
				defer s.Close()
				for {
					fileID, chunkIdx, data, err := protocol.ReadChunk(s)
					if err != nil {
						return // EOF = stream done
					}
					val, ok := fileMap.Load(fileID)
					if !ok {
						streamErr <- fmt.Errorf("unknown fileID %d", fileID)
						return
					}
					fs := val.(*fileState)
					offset := chunkIdx * protocol.ChunkSize
					fs.mu.Lock()
					_, werr := fs.outFile.WriteAt(data, offset)
					fs.mu.Unlock()
					if werr != nil {
						streamErr <- werr
						return
					}
					fs.checkpoint.Mark(chunkIdx)
					fs.progress.Add(int64(len(data)))
					monitor.Global.BytesAcked.Add(int64(len(data)))
				}
			}(stream)
		}
	}()

	// Process FileInfo messages sequentially on control stream
	for {
		msgType, raw, err := protocol.ReadMsg(ctrl)
		if err != nil {
			return err
		}
		if msgType == protocol.MsgDone {
			break
		}
		if msgType != protocol.MsgFileInfo {
			continue
		}

		var info protocol.FileInfo
		if err := json.Unmarshal(raw, &info); err != nil {
			return err
		}

		// Sanitize path
		outPath := filepath.Join(sessionDir, filepath.FromSlash(info.Name))
		os.MkdirAll(filepath.Dir(outPath), 0755)

		// Check resume
		ckpt, _ := checkpoint.Load(sessionDir, hello.SessionID, info.Name,
			info.ChunkCount, info.Size)
		resumeChunk := ckpt.ResumeChunk()

		skip := ckpt.Complete()
		if err := protocol.WriteMsg(ctrl, protocol.MsgFileAck, protocol.FileAck{
			ID:          info.ID,
			ResumeChunk: resumeChunk,
			Skip:        skip,
		}); err != nil {
			return err
		}

		if skip {
			monitor.Global.FilesComplete.Add(1)
			continue
		}

		f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		f.Truncate(info.Size)

		fp := display.AddFile(info.Name, info.Size-resumeChunk*protocol.ChunkSize)
		fs := &fileState{info: info, outFile: f, checkpoint: ckpt, progress: fp}
		fileMap.Store(uint16(info.ID), fs)
	}

	// Wait for all writes
	select {
	case err := <-streamErr:
		return err
	case <-time.After(5 * time.Second):
	}

	// Close files and clean up checkpoints
	fileMap.Range(func(_, val any) bool {
		fs := val.(*fileState)
		fs.outFile.Close()
		if fs.checkpoint.Complete() {
			fs.checkpoint.Delete()
			monitor.Global.FilesComplete.Add(1)
		}
		return true
	})

	return nil
}

// BytesCounter wraps atomic counter for tracking
type BytesCounter struct {
	n atomic.Int64
}

func (c *BytesCounter) Add(n int64) { c.n.Add(n) }
func (c *BytesCounter) Load() int64 { return c.n.Load() }
