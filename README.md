<p align="center">
  <br>
  <pre>
██     ██  █████  ██ ██     ██  █████  ██
██     ██ ██   ██ ██ ██     ██ ██   ██ ██
██  █  ██ ███████ ██ ██  █  ██ ███████ ██
██ ███ ██ ██   ██ ██ ██ ███ ██ ██   ██ ██
 ███ ███  ██   ██ ██  ███ ███  ██   ██ ██
  </pre>
  <br><br>
  <strong>ส่งไว รับไว ไม่ง้อ TCP</strong>
  <br>
  ส่งไฟล์ข้ามเครื่องด้วย QUIC — เร็วกว่า scp หลายเท่า
  <br><br>
  <a href="https://github.com/killernay/waiwai/actions/workflows/ci.yml"><img src="https://github.com/killernay/waiwai/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/killernay/waiwai/releases"><img src="https://img.shields.io/github/v/release/killernay/waiwai?include_prereleases&style=flat-square" alt="Release"></a>
  <a href="https://goreportcard.com/report/github.com/killernay/waiwai"><img src="https://goreportcard.com/badge/github.com/killernay/waiwai?style=flat-square" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License"></a>
</p>

---

## ทำไมต้อง waiwai?

เครื่องมืออย่าง `scp` และ `rsync` ใช้ TCP ซึ่งส่งได้ทีละ stream เดียว พอเจอ latency สูงหรือ packet loss ความเร็วตกทันที **waiwai** ใช้ [QUIC](https://www.rfc-editor.org/rfc/rfc9000.html) ส่งพร้อมกัน 8–32 streams ผ่าน UDP ทำให้ใช้ bandwidth ได้เต็มท่อแม้ส่งข้ามประเทศ

| สถานการณ์ | scp | waiwai (16 streams) | เร็วขึ้น |
|---|---|---|---|
| LAN (< 1 ms RTT) | ~900 MB/s | ~900 MB/s | 1x |
| WAN 150 ms RTT | ~20 MB/s | ~150 MB/s | **7.5x** |
| WAN 200 ms + 1% loss | ~5 MB/s | ~80 MB/s | **16x** |

## คุณสมบัติ

| คุณสมบัติ | รายละเอียด |
|---|---|
| **Multi-stream QUIC** | ส่งพร้อมกัน 8–32 streams ในการเชื่อมต่อเดียว |
| **Resume** | หลุดกลางทาง เชื่อมใหม่แล้วส่งต่อจากจุดเดิมได้เลย |
| **ส่งได้ทั้งไฟล์และโฟลเดอร์** | ส่งทั้งโฟลเดอร์พร้อมโครงสร้างในคำสั่งเดียว |
| **จำกัด bandwidth** | `--rate 50` จำกัดที่ 50 MB/s ไม่กิน bandwidth คนอื่น |
| **ดูสถานะ realtime** | JSON `/status` + Prometheus `/metrics` ทั้งฝั่งส่งและรับ |
| **เข้ารหัสทุก byte** | ใช้ TLS 1.3 ผ่าน QUIC โดยอัตโนมัติ |
| **ไม่ต้องตั้งค่า** | สร้าง cert เองอัตโนมัติ — สั่งรันแล้วส่งได้เลย |
| **ทุก OS** | Linux, macOS, Windows — ไฟล์เดียว ไม่ต้องติดตั้งอะไรเพิ่ม |

---

# สำหรับผู้ใช้ — โหลดแล้วใช้ได้เลย

ไม่ต้อง build เอง ไม่ต้องติดตั้ง Go แค่โหลด binary แล้วรันได้ทันที

## ดาวน์โหลด

ไปที่หน้า [**Releases**](https://github.com/killernay/waiwai/releases/latest) แล้วเลือกตาม OS ของคุณ:

| ระบบปฏิบัติการ | สถาปัตยกรรม | ดาวน์โหลด |
|---|---|---|
| **Windows** | x64 (ทั่วไป) | [`waiwai_windows_amd64.zip`](https://github.com/killernay/waiwai/releases/latest/download/waiwai_0.1.0_windows_amd64.zip) |
| **Windows** | ARM64 | [`waiwai_windows_arm64.zip`](https://github.com/killernay/waiwai/releases/latest/download/waiwai_0.1.0_windows_arm64.zip) |
| **macOS** | Apple Silicon (M1/M2/M3/M4) | [`waiwai_darwin_arm64.tar.gz`](https://github.com/killernay/waiwai/releases/latest/download/waiwai_0.1.0_darwin_arm64.tar.gz) |
| **macOS** | Intel | [`waiwai_darwin_amd64.tar.gz`](https://github.com/killernay/waiwai/releases/latest/download/waiwai_0.1.0_darwin_amd64.tar.gz) |
| **Linux** | x64 (ทั่วไป) | [`waiwai_linux_amd64.tar.gz`](https://github.com/killernay/waiwai/releases/latest/download/waiwai_0.1.0_linux_amd64.tar.gz) |
| **Linux** | ARM64 | [`waiwai_linux_arm64.tar.gz`](https://github.com/killernay/waiwai/releases/latest/download/waiwai_0.1.0_linux_arm64.tar.gz) |

### วิธีติดตั้ง — Windows

```powershell
# 1. โหลดและแตก zip
Invoke-WebRequest -Uri "https://github.com/killernay/waiwai/releases/latest/download/waiwai_0.1.0_windows_amd64.zip" -OutFile waiwai.zip
Expand-Archive waiwai.zip -DestinationPath .

# 2. ใช้งานได้เลย
.\waiwai.exe --help
```

### วิธีติดตั้ง — macOS / Linux

```bash
# 1. โหลดและแตกไฟล์ (ตัวอย่าง: macOS Apple Silicon)
curl -L https://github.com/killernay/waiwai/releases/latest/download/waiwai_0.1.0_darwin_arm64.tar.gz | tar xz

# 2. ย้ายเข้า PATH (ไม่บังคับ แต่จะเรียกใช้ง่ายขึ้น)
sudo mv waiwai /usr/local/bin/

# 3. ใช้งานได้เลย
waiwai --help
```

## เริ่มต้นใช้งาน

### macOS / Linux

**ขั้นตอนที่ 1 — เปิดรับไฟล์** (รันบนเครื่องปลายทางก่อน):

```bash
waiwai recv --out /data/incoming
```

**ขั้นตอนที่ 2 — ส่งไฟล์** (รันบนเครื่องต้นทาง):

```bash
waiwai send photo.jpg 192.168.1.100:4242
```

เสร็จ! ไฟล์จะอยู่ที่ `/data/incoming`

### Windows

**ขั้นตอนที่ 1 — เปิดรับไฟล์** (รันบนเครื่องปลายทางก่อน):

```powershell
# รับไฟล์ไว้ที่โฟลเดอร์ Downloads\incoming
.\waiwai.exe recv --out C:\Users\YourName\Downloads\incoming
```

> Windows อาจถาม **Windows Firewall** ให้กด **Allow access** เพื่อเปิดให้รับข้อมูลผ่าน UDP ได้

**ขั้นตอนที่ 2 — ส่งไฟล์** (รันบนเครื่องต้นทาง):

```powershell
# ส่งไฟล์ไปยัง IP ของเครื่องรับ
.\waiwai.exe send photo.jpg 192.168.1.100:4242
```

> **หา IP เครื่อง Windows:** เปิด PowerShell พิมพ์ `ipconfig` แล้วดูที่ **IPv4 Address**

เสร็จ! ไฟล์จะอยู่ที่ `C:\Users\YourName\Downloads\incoming`

## ตัวอย่างการใช้งาน

### macOS / Linux

```bash
# ส่งไฟล์เดียว
waiwai send photo.jpg 192.168.1.100:4242

# ส่งทั้งโฟลเดอร์ ใช้ 16 streams
waiwai send -s 16 ./footage/ 192.168.1.100:4242

# จำกัด bandwidth ที่ 50 MB/s
waiwai send --rate 50 bigfile.bin 192.168.1.100:4242

# ส่งต่อจากที่หลุดไป (resume)
waiwai send --resume myjob bigfile.bin 192.168.1.100:4242

# รับไฟล์บน port อื่น
waiwai recv --listen 0.0.0.0:5000 --out /data/incoming

# เปิดดูสถานะ realtime ทั้งสองฝั่ง
waiwai send --monitor :9090 ./footage/ 192.168.1.100:4242
waiwai recv --out /data --monitor :9091

# เช็คสถานะการส่ง
waiwai status localhost:9090
```

### Windows (PowerShell)

```powershell
# ส่งไฟล์เดียว
.\waiwai.exe send photo.jpg 192.168.1.100:4242

# ส่งทั้งโฟลเดอร์ ใช้ 16 streams
.\waiwai.exe send -s 16 .\footage\ 192.168.1.100:4242

# จำกัด bandwidth ที่ 50 MB/s
.\waiwai.exe send --rate 50 bigfile.bin 192.168.1.100:4242

# ส่งต่อจากที่หลุดไป (resume)
.\waiwai.exe send --resume myjob bigfile.bin 192.168.1.100:4242

# รับไฟล์ไว้ที่โฟลเดอร์ที่ต้องการ
.\waiwai.exe recv --listen 0.0.0.0:5000 --out C:\data\incoming

# เปิดดูสถานะ realtime ทั้งสองฝั่ง
.\waiwai.exe send --monitor :9090 .\footage\ 192.168.1.100:4242
.\waiwai.exe recv --out C:\data --monitor :9091

# เช็คสถานะการส่ง
.\waiwai.exe status localhost:9090
```

## คำสั่งทั้งหมด

### `waiwai send` — ส่งไฟล์

```
waiwai send [flags] <ไฟล์|โฟลเดอร์>... <host:port>
```

| Flag | ค่าเริ่มต้น | คำอธิบาย |
|---|---|---|
| `-s, --streams` | `8` | จำนวน stream ที่ส่งพร้อมกัน |
| `-r, --rate` | `0` (ไม่จำกัด) | จำกัด bandwidth (หน่วย MB/s) |
| `--resume` | | session ID สำหรับส่งต่อจากที่หลุด |
| `--monitor` | | เปิด metrics server เช่น `:9090` |
| `--cert` | | ไฟล์ TLS certificate (ไม่บังคับ) |
| `--key` | | ไฟล์ TLS private key (ไม่บังคับ) |

### `waiwai recv` — รับไฟล์

```
waiwai recv [flags]
```

| Flag | ค่าเริ่มต้น | คำอธิบาย |
|---|---|---|
| `-l, --listen` | `0.0.0.0:4242` | address ที่จะ listen |
| `-o, --out` | `.` (โฟลเดอร์ปัจจุบัน) | โฟลเดอร์ที่จะบันทึกไฟล์ |
| `--monitor` | | เปิด metrics server เช่น `:9091` |
| `--cert` | | ไฟล์ TLS certificate (ไม่บังคับ) |
| `--key` | | ไฟล์ TLS private key (ไม่บังคับ) |

### `waiwai status` — ดูสถานะ

```
waiwai status <monitor-addr>
```

```
⚡ waiwai status @ 192.168.1.100:9090
  [████████████████░░░░░░░░░░░░░░] 53.2%
  ความเร็ว:   142.50 MB/s
  ส่งแล้ว:    5324.10 MB
  ไฟล์:       3 / 7
  Streams:    16 active
  เหลืออีก:   33s
```

## Monitoring Endpoints

ทั้งฝั่งส่งและรับจะเปิด HTTP endpoint เมื่อใส่ `--monitor`:

| Endpoint | รูปแบบ | คำอธิบาย |
|---|---|---|
| `GET /status` | JSON | สถานะแบบอ่านง่าย |
| `GET /metrics` | Prometheus text | สำหรับ scrape ด้วย Prometheus / Grafana |
| `GET /health` | Plain text | เช็คว่าทำงานอยู่ (`ok`) |

## ข้อกำหนด

- เปิด **UDP port 4242** ระหว่างเครื่องส่งและรับ (port เริ่มต้น)
- ใช้กับ [Tailscale](https://tailscale.com) ได้เลย ไม่ต้อง forward port

> **หมายเหตุเรื่อง 5G / มือถือ:** waiwai ใช้ QUIC ซึ่งทำงานบน UDP — ผู้ให้บริการมือถือบางราย (AIS, TRUE, DTAC) อาจ throttle หรือ block UDP traffic ทำให้ส่งช้าผิดปกติหรือเชื่อมต่อไม่ได้ หากเจอปัญหานี้ แนะนำให้ใช้ผ่าน WiFi หรือเครือข่ายแบบมีสายแทน

---

# สำหรับนักพัฒนา — Build และร่วมพัฒนา

## สิ่งที่ต้องมี

- **Go 1.22+**
- **goreleaser** (ไม่บังคับ สำหรับ build ทุก platform)
- **golangci-lint** (ไม่บังคับ สำหรับตรวจสอบโค้ด)

## ติดตั้งจาก source

```bash
go install github.com/killernay/waiwai/cmd/waiwai@latest
```

## Build จาก source

```bash
git clone https://github.com/killernay/waiwai.git
cd waiwai

make build          # → bin/waiwai
make test           # รัน test พร้อม race detector
make lint           # ตรวจสอบโค้ดด้วย golangci-lint
```

## โครงสร้างโปรเจค

```
waiwai/
├── cmd/waiwai/main.go            # จุดเริ่มต้น CLI (cobra)
├── pkg/protocol/protocol.go      # Wire format: control messages + binary chunks
├── internal/
│   ├── transfer/transfer.go      # engine หลัก ส่ง/รับ
│   ├── transfer/tls_gen.go       # สร้าง self-signed TLS cert
│   ├── checkpoint/checkpoint.go  # เก็บสถานะสำหรับ resume
│   ├── throttle/throttle.go      # token-bucket bandwidth limiter
│   ├── monitor/monitor.go        # HTTP server สำหรับ metrics
│   └── ui/ui.go                  # progress bar บน terminal
├── .goreleaser.yaml              # pipeline สำหรับ build ทุก platform
└── .github/workflows/
    ├── ci.yml                    # รัน test ทุก PR
    └── release.yml               # build + release เมื่อ push tag
```

## การออกแบบ Protocol

```
Stream 0   ─── Control (JSON มี length prefix) ─────────────────────
                 hello → accept → file_info → file_ack → ... → done

Stream 1+  ─── Data (binary chunk header + payload) ────────────────
                 [fileID:2][chunkIdx:8][dataLen:4][sha256:32][data...]
```

- **Control stream** ส่ง handshake, metadata ของไฟล์ และ ack เป็น JSON
- **Data streams** (1–32 streams) ส่ง chunk ของไฟล์พร้อม binary header ขนาด 46 bytes
- ทุก chunk ถูกตรวจสอบ SHA-256 เพื่อความถูกต้อง
- Resume ทำงานโดยบันทึก chunk ที่รับแล้วลง checkpoint file

## คำสั่ง Makefile

| คำสั่ง | คำอธิบาย |
|---|---|
| `make build` | build binary ไปที่ `bin/waiwai` |
| `make test` | รัน test พร้อม race detector |
| `make lint` | ตรวจสอบโค้ดด้วย golangci-lint |
| `make snapshot` | build ทุก platform แบบ local (ไม่ publish) |
| `make tag` | สร้าง tag แล้ว push — GitHub Actions ทำ release ให้อัตโนมัติ |
| `make testfile` | สร้างไฟล์ทดสอบขนาด 1 GB |
| `make bench-scp` | วัดความเร็ว scp |
| `make bench-waiwai` | วัดความเร็ว waiwai |
| `make clean` | ลบไฟล์ที่ build ออก |

## วิธี Release

Release ทำงานอัตโนมัติผ่าน GitHub Actions:

```bash
# สร้าง tag → GitHub Actions จะ build ทุก platform ให้เอง
make tag
# ใส่เวอร์ชัน: v0.2.0
# → binary สำหรับ linux/darwin/windows (amd64+arm64) ขึ้นบน GitHub Releases อัตโนมัติ
```

## ร่วมพัฒนา

ยินดีรับ contribution ทุกรูปแบบ! แนะนำให้เปิด issue ก่อนเพื่อพูดคุยกัน

1. Fork repo นี้
2. สร้าง branch ใหม่ (`git checkout -b feature/amazing`)
3. Commit การเปลี่ยนแปลง (`git commit -m 'เพิ่มฟีเจอร์ amazing'`)
4. Push ขึ้น branch (`git push origin feature/amazing`)
5. เปิด Pull Request

---

## สัญญาอนุญาต

[MIT](LICENSE) — ใช้ได้ตามสบาย
