// waiwai — ส่งไว รับไว ไม่ง้อ TCP
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/killernay/waiwai/internal/monitor"
	"github.com/killernay/waiwai/internal/transfer"
)

var version = "dev"

const banner = `
  ██     ██  █████  ██ ██     ██  █████  ██
  ██     ██ ██   ██ ██ ██     ██ ██   ██ ██
  ██  █  ██ ███████ ██ ██  █  ██ ███████ ██
  ██ ███ ██ ██   ██ ██ ██ ███ ██ ██   ██ ██
   ███ ███  ██   ██ ██  ███ ███  ██   ██ ██
  ไวไว · ส่งไว รับไว ไม่ง้อ TCP · QUIC-powered
`

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "waiwai",
		Short:   "ไวไว — fast file transfer over QUIC",
		Version: version,
		Long: banner + `
ส่งไฟล์และโฟลเดอร์ผ่าน QUIC protocol
เร็วกว่า scp/rsync บน WAN หลายเท่า ทำงานได้หลัง NAT ทั้งสองฝั่ง

Examples:
  waiwai send photo.jpg 100.64.1.2:4242
  waiwai send ./footage/ 100.64.1.2:4242
  waiwai send --rate 50 --streams 16 bigfile.bin 100.64.1.2:4242
  waiwai recv --out /data/incoming
  waiwai status localhost:9090`,
	}
	root.AddCommand(sendCmd(), recvCmd(), statusCmd(), versionCmd())
	return root
}

func sendCmd() *cobra.Command {
	var streams int
	var rateMB int64
	var sessionID, monitorAddr, certFile, keyFile string

	cmd := &cobra.Command{
		Use:   "send [flags] <ไฟล์|โฟลเดอร์>... <host:port>",
		Short: "ส่งไฟล์หรือโฟลเดอร์ไปยังปลายทาง ⚡",
		Example: `  waiwai send photo.jpg 100.64.1.2:4242
  waiwai send ./footage/ 100.64.1.2:4242
  waiwai send --rate 50 bigfile.bin 100.64.1.2:4242
  waiwai send --streams 16 bigfile.bin 100.64.1.2:4242
  waiwai send --resume myjob bigfile.bin 100.64.1.2:4242
  waiwai send --monitor :9090 ./footage/ 100.64.1.2:4242`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := args[:len(args)-1]
			addr := args[len(args)-1]
			if !strings.Contains(addr, ":") {
				addr = fmt.Sprintf("%s:4242", addr)
			}
			opts := transfer.SendOptions{
				Addr: addr, Paths: paths, NumStreams: streams,
				RateLimitMB: rateMB, SessionID: sessionID, MonitorAddr: monitorAddr,
			}
			if certFile != "" && keyFile != "" {
				tlsCfg, err := transfer.LoadTLS(certFile, keyFile)
				if err != nil {
					return fmt.Errorf("โหลด TLS ไม่ได้: %w", err)
				}
				opts.TLSConfig = tlsCfg
			}
			fmt.Printf("⚡ waiwai ส่ง → %s  [%d streams]\n", addr, streams)
			if rateMB > 0 {
				fmt.Printf("🚦 จำกัด bandwidth: %d MB/s\n", rateMB)
			}
			if sessionID != "" {
				fmt.Printf("🔁 resume session: %s\n", sessionID)
			}
			return transfer.Send(context.Background(), opts)
		},
	}
	cmd.Flags().IntVarP(&streams, "streams", "s", 8, "จำนวน parallel streams")
	cmd.Flags().Int64VarP(&rateMB, "rate", "r", 0, "จำกัด bandwidth MB/s (0 = ไม่จำกัด)")
	cmd.Flags().StringVar(&sessionID, "resume", "", "session ID สำหรับ resume")
	cmd.Flags().StringVar(&monitorAddr, "monitor", "", "เปิด metrics เช่น :9090")
	cmd.Flags().StringVar(&certFile, "cert", "", "TLS certificate file")
	cmd.Flags().StringVar(&keyFile, "key", "", "TLS key file")
	return cmd
}

func recvCmd() *cobra.Command {
	var listenAddr, outDir, monitorAddr, certFile, keyFile string

	cmd := &cobra.Command{
		Use:   "recv [flags]",
		Short: "รอรับไฟล์ (รันบนเครื่องปลายทางก่อน) 📥",
		Example: `  waiwai recv
  waiwai recv --listen 0.0.0.0:4242 --out /data/incoming
  waiwai recv --out /data --monitor :9091`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := transfer.ReceiveOptions{
				ListenAddr: listenAddr, OutDir: outDir, MonitorAddr: monitorAddr,
			}
			if certFile != "" && keyFile != "" {
				tlsCfg, err := transfer.LoadTLS(certFile, keyFile)
				if err != nil {
					return fmt.Errorf("โหลด TLS ไม่ได้: %w", err)
				}
				opts.TLSConfig = tlsCfg
			}
			return transfer.Receive(context.Background(), opts)
		},
	}
	cmd.Flags().StringVarP(&listenAddr, "listen", "l", "0.0.0.0:4242", "address ที่จะ listen")
	cmd.Flags().StringVarP(&outDir, "out", "o", ".", "โฟลเดอร์ที่จะบันทึกไฟล์")
	cmd.Flags().StringVar(&monitorAddr, "monitor", "", "เปิด metrics เช่น :9091")
	cmd.Flags().StringVar(&certFile, "cert", "", "TLS certificate file")
	cmd.Flags().StringVar(&keyFile, "key", "", "TLS key file")
	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <monitor-addr>",
		Short: "ดูสถานะการส่ง/รับแบบ realtime 📊",
		Example: `  waiwai status localhost:9090
  waiwai status 100.64.1.2:9091`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := args[0]
			if !strings.HasPrefix(addr, "http") {
				addr = "http://" + addr
			}
			resp, err := http.Get(addr + "/status")
			if err != nil {
				return fmt.Errorf("เชื่อมต่อ monitor ไม่ได้: %w", err)
			}
			defer resp.Body.Close()

			var snap monitor.Snapshot
			if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
				return err
			}
			pct := 0.0
			if snap.TotalBytes > 0 {
				pct = float64(snap.BytesSent) / float64(snap.TotalBytes) * 100
			}
			filled := int(pct / 100 * 30)
			bar := strings.Repeat("█", filled) + strings.Repeat("░", 30-filled)

			fmt.Printf("\n⚡ waiwai status @ %s\n", args[0])
			fmt.Printf("  [%s] %.1f%%\n", bar, pct)
			fmt.Printf("  ความเร็ว:   %.2f MB/s\n", snap.ThroughputMBs)
			fmt.Printf("  ส่งแล้ว:    %.2f MB\n", float64(snap.BytesSent)/1e6)
			fmt.Printf("  ไฟล์:       %d / %d\n", snap.FilesComplete, snap.FilesTotal)
			fmt.Printf("  Streams:    %d active\n", snap.ActiveStreams)
			fmt.Printf("  เวลาทำงาน:  %.0fs\n", snap.UptimeSeconds)
			if snap.ETA != "" {
				fmt.Printf("  เหลืออีก:   %s\n", snap.ETA)
			}
			if snap.Errors > 0 {
				fmt.Printf("  ⚠️  Errors: %d\n", snap.Errors)
			}
			fmt.Println()
			return nil
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "แสดง version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("waiwai %s\n", version)
		},
	}
}
