BINARY  := waiwai
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"

.PHONY: build test lint clean snapshot tag testfile bench-scp bench-waiwai

build:
	go mod tidy
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/waiwai

# ทดสอบ release แบบ local ไม่ publish
snapshot:
	goreleaser release --snapshot --clean

# release จริงผ่าน GitHub Actions — push tag แล้วรอเลย
# หรือรันเองถ้ามี GITHUB_TOKEN
release-local:
	goreleaser release --clean

# สร้าง tag แล้ว push → GitHub Actions ทำต่อเอง
tag:
	@read -p "Version (e.g. v0.1.0): " VERSION; \
	git tag -a $$VERSION -m "Release $$VERSION"; \
	git push origin $$VERSION; \
	echo "✅ Tag $$VERSION pushed — รอ Actions build เลยครับ"

test:
	go test ./... -race -timeout 60s

lint:
	golangci-lint run ./...

testfile:
	dd if=/dev/urandom of=/tmp/testfile_1gb.bin bs=1M count=1024 status=progress

bench-scp:
	time scp /tmp/testfile_1gb.bin $(HOST):/tmp/

bench-waiwai:
	time ./bin/waiwai send -s 16 /tmp/testfile_1gb.bin $(ADDR)

clean:
	rm -rf bin/ dist/
