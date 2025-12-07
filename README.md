# web-test

## Build

sudo visudo

在檔案最後加入（替換 rs 為你的實際使用者名）：
rs ALL=(ALL) NOPASSWD: /home/rs/web_test/run_task.sh


### 1. 創建數據目錄
```bash
mkdir -p $HOME/kvrocks_data
```

### 2. 啟動 KVRocks 容器
```bash
docker run -d \
  --name kvrocks \
  -p 6379:6666 \
  -v $HOME/kvrocks_data:/var/lib/kvrocks \
  apache/kvrocks:latest
```

## Run

### 啟動應用
```bash
go run cmd/main.go
```

## 清理

### 停止容器
```bash
docker stop kvrocks
```

### 移除容器
```bash
docker rm kvrocks
```

### 移除數據目錄（可選）
```bash
rm -rf $HOME/kvrocks_data
```