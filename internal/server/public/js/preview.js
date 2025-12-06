document.addEventListener("DOMContentLoaded", () => {
    const search = new URLSearchParams(window.location.search);
    const taskId = search.get("taskId");

    const idEl = document.getElementById("task-id");
    const statusEl = document.getElementById("task-status");
    const timeEl = document.getElementById("task-time");
    const failedEl = document.getElementById("task-failed"); 
    const logsEl = document.getElementById("task-logs");
    const errorBanner = document.getElementById("error-banner");
    
    // 獲取選單元素
    const selectorContainerEl = document.getElementById("failed-test-selector-container");
    const selectorEl = document.getElementById("failed-test-selector"); 
    // 獲取按鈕元素
    const downloadBtn = document.getElementById("download-btn");
    
    // 儲存任務資料
    let currentTaskData = null; 

    if (!taskId) {
        showError("缺少任務 ID");
        logsEl.textContent = "無法載入";
        return;
    }

    idEl.textContent = taskId;
    document.title = `任務 #${taskId}`;

    fetchTask(taskId);

    // ==========================================
    //  事件監聽：下載按鈕
    // ==========================================
    if (downloadBtn) {
        downloadBtn.addEventListener("click", () => {
            // 1. 基本檢查
            if (!currentTaskData || !currentTaskData.failed_tests) {
                alert("無任務資料或無失敗測試");
                return;
            }

            // 2. 獲取選單當前的 index
            const index = selectorEl.value;
            
            // 3. 取得對應的測試名稱 (failedTestName)
            const failedTestName = currentTaskData.failed_tests[index];

            if (!failedTestName) {
                alert("無法確認測試名稱");
                return;
            }

            // 4. 執行下載
            downloadCase(taskId, failedTestName);
        });
    }

    // ==========================================
    //  核心函式
    // ==========================================

    async function fetchTask(id) {
        try {
            const response = await fetch(`/api/task/${encodeURIComponent(id)}`);
            const payload = await response.json().catch(() => null);

            if (!response.ok) {
                throw new Error((payload && payload.error) || "無法取得任務詳細資料");
            }
            
            currentTaskData = payload; 
            applyTask(payload);
        } catch (err) {
            showError(err.message || "載入失敗");
            logsEl.textContent = "無法載入任務資料";
        }
    }

    function applyTask(task) {
        const status = task.status || "-";
        const normalized = status.toLowerCase();
        
        const statusColorMap = {
            success: "#2e7d32",
            failed: "#c62828",
            running: "#fb8c00",
            queueing: "#757575"
        };
        statusEl.textContent = status;
        statusEl.style.color = statusColorMap[normalized] || "#311b92";

        const ts = Number(task.timestamp);
        if (ts) {
            const date = new Date(ts * 1000);
            timeEl.textContent = date.toLocaleString("zh-TW", { hour12: false });
        } else {
            timeEl.textContent = "-";
        }

        // --- 處理 Log 與 Failed Tests ---
        const failedTests = Array.isArray(task.failed_tests) ? task.failed_tests : [];
        
        let logArray = [];
        if (Array.isArray(task.logs)) {
            logArray = task.logs;
        } else if (typeof task.logs === 'string') {
            logArray = [task.logs];
        }

        if (failedEl) {
            failedEl.textContent = failedTests.length > 0 ? `共 ${failedTests.length} 個失敗` : "-";
        }

        selectorEl.innerHTML = ''; 

        // 邏輯：失敗且有具體測試項目才顯示選單與下載按鈕
        if (normalized === 'failed' && failedTests.length > 0) {
            
            selectorContainerEl.style.display = 'block'; // 顯示容器(包含選單與按鈕)
            
            failedTests.forEach((testName, index) => {
                const option = document.createElement('option');
                option.value = index; 
                option.textContent = (testName && String(testName)) || `[測試 ${index + 1}]`; 
                selectorEl.appendChild(option);
            });

            // 監聽選單切換 (顯示 Log)
            selectorEl.removeEventListener('change', handleSelectorChange);
            selectorEl.addEventListener('change', handleSelectorChange);

            // 預設顯示第一筆 Log
            if (logArray.length > 0) {
                displayFullLog(logArray[0]);
            } else {
                displayFullLog("無 Log 資料");
            }

        } else {
            // 成功或無詳細失敗名單 -> 隱藏選單與按鈕
            selectorContainerEl.style.display = 'none';

            const joinedLogs = logArray.join("\n\n" + "-".repeat(40) + "\n\n");
            displayFullLog(joinedLogs);
        }
    }
    
    function handleSelectorChange(e) {
        if (!currentTaskData) return;
        const index = parseInt(e.target.value, 10);
        const logs = currentTaskData.logs || [];

        if (logs[index] !== undefined) {
            displayFullLog(logs[index]);
        } else {
            displayFullLog("找不到此測試對應的 Log");
        }
    }

    /**
     * 下載案件 API 呼叫
     * URL 格式: /api/task/download?taskId=XXX&failedTestName=YYY
     */
    async function downloadCase(tId, tName) {
        // 使用 encodeURIComponent 確保特殊字元 (如 / 或空格) 不會破壞 URL
        const url = `/api/download/single/${tId}/${encodeURIComponent(tName)}`;
        
        const originalText = downloadBtn.innerHTML;
        downloadBtn.innerHTML = `<span class="icon">⏳</span> 處理中...`;
        downloadBtn.disabled = true;

        try {
            const response = await fetch(url);

            if (!response.ok) {
                const errJson = await response.json().catch(() => ({}));
                throw new Error(errJson.error || "下載請求失敗");
            }

            // 轉換為 Blob 進行下載
            const blob = await response.blob();
            
            // 嘗試解析檔名
            const disposition = response.headers.get('Content-Disposition');
            // 預設檔名格式: taskId_failedTestName.log (可根據後端回傳調整)
            let filename = `${tId}_${tName.replace(/[^a-zA-Z0-9]/g, '_')}.log`; 

            if (disposition && disposition.includes('attachment')) {
                const filenameRegex = /filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/;
                const matches = filenameRegex.exec(disposition);
                if (matches != null && matches[1]) { 
                    filename = matches[1].replace(/['"]/g, '');
                }
            }

            // 觸發瀏覽器下載
            const downloadUrl = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = downloadUrl;
            a.download = filename;
            document.body.appendChild(a);
            a.click();
            
            window.URL.revokeObjectURL(downloadUrl);
            document.body.removeChild(a);

        } catch (err) {
            console.error(err);
            alert(`下載錯誤: ${err.message}`);
        } finally {
            downloadBtn.innerHTML = originalText;
            downloadBtn.disabled = false;
        }
    }

    function displayFullLog(logContent) {
        const rawLogs = String(logContent || "無 Log"); 
        const cleanLogs = rawLogs.replace(/\r\n/g, "\n");
        logsEl.textContent = cleanLogs;
    }

    function showError(msg) {
        errorBanner.style.display = "block";
        errorBanner.textContent = msg;
    }
});