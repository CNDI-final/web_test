# Build Stage
FROM golang:alpine AS builder

WORKDIR /app

# 複製後端依賴並下載
COPY go.mod go.sum ./
RUN go mod download
RUN go mod tidy

# 複製後端原始碼並編譯
COPY . .
RUN CGO_ENABLED=0 go build -o main main.go

# Run Stage (使用輕量級 Alpine Linux)
FROM alpine:latest

WORKDIR /root/

# 把編譯好的後端主程式複製過來
COPY --from=builder /app/main .

# ★ 重要：把前端檔案也複製過來，不然網頁會 404
COPY frontend ./frontend
COPY config ./config

# 預設指令 (雖然 Compose 會覆蓋它，但留著當預設也好)
CMD ["./main"]