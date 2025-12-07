# web-test

## Build
```bash
sudo visudo
```

在檔案最後加入（替換 rs 為你的實際使用者名）：
```bash
rs ALL=(ALL) NOPASSWD: /home/rs/web_test/run_task.sh
```

### 啟動 KVRocks 容器
```bash
make
```

## Run

### 啟動應用
```bash
go run cmd/main.go
```

## 清理


### 移除容器
```bash
make stop
```

### 移除數據目錄（可選）
```bash
sudo make clean
```