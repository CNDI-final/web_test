# ===========================
# 變數設定
# ===========================
DIR_NAME := kvrocks_data
FILE_NAME := kvrocks.conf
DOWNLOAD_URL := https://raw.githubusercontent.com/apache/kvrocks/unstable/kvrocks.conf
CONTAINER_NAME := kvrocks

.PHONY: all run stop clean

# 預設目標
all: run

# ===========================
# Run: 一條龍執行 (建資料夾 -> 下載 -> 改權限 -> 啟動)
# ===========================
run:
	@echo "⚙️  正在準備環境..."
	
	@# 1. 建立資料夾 & 設定權限 777
	@mkdir -p $(DIR_NAME)
	@chmod 777 $(DIR_NAME)
	
	@# 2. 檢查設定檔，沒有就下載
	@if [ ! -f $(DIR_NAME)/$(FILE_NAME) ]; then \
	    echo "⬇️  設定檔缺失，正在下載..."; \
	    curl -o $(DIR_NAME)/$(FILE_NAME) $(DOWNLOAD_URL); \
	    chmod 644 $(DIR_NAME)/$(FILE_NAME); \
	fi

	@echo "🚀 正在啟動 Docker 容器..."
	
	@# 3. 刪除舊容器 (忽略錯誤)
	-docker rm -f $(CONTAINER_NAME) 2>/dev/null
	
	@# 4. 啟動容器
	docker run -d \
	  --name $(CONTAINER_NAME) \
	  --restart always \
	  -p 6379:6666 \
	  -v $(shell pwd)/$(DIR_NAME):/var/lib/kvrocks \
	  apache/kvrocks:latest
	  
	@echo "🎉 Kvrocks 啟動成功！(資料夾: $(DIR_NAME))"

# ===========================
# Stop: 單純停止容器
# ===========================
stop:
	@echo "🛑 停止容器..."
	-docker rm -f $(CONTAINER_NAME) 2>/dev/null

# ===========================
# Clean: 先停止，再刪除檔案
# ===========================
clean: stop
	@echo "🧹 清除資料夾與檔案..."
	rm -rf $(DIR_NAME)
	@echo "✨ 清除完畢。"
