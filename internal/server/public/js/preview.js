document.addEventListener("DOMContentLoaded", () => {
    const search = new URLSearchParams(window.location.search);
    const taskId = search.get("taskId");

    const idEl = document.getElementById("task-id");
    const statusEl = document.getElementById("task-status");
    const timeEl = document.getElementById("task-time");
    const failedEl = document.getElementById("task-failed");
    const logsEl = document.getElementById("task-logs");
    const errorBanner = document.getElementById("error-banner");

    if (!taskId) {
        showError("缺少任務 ID");
        logsEl.textContent = "無法載入";
        return;
    }

    idEl.textContent = taskId;
    document.title = `任務 #${taskId}`;

    fetchTask(taskId);

    async function fetchTask(id) {
        try {
            const response = await fetch(`/api/task/${encodeURIComponent(id)}`);
            const payload = await response.json().catch(() => null);

            if (!response.ok) {
                throw new Error((payload && payload.error) || "無法取得任務詳細資料");
            }

            applyTask(payload);
        } catch (err) {
            showError(err.message || "載入失敗");
            logsEl.textContent = "無法載入";
        }
    }

    function applyTask(task) {
        const status = task.status || "-";
        const normalized = status.toLowerCase();
        const statusColorMap = {
            success: "#2e7d32",
            failed: "#c62828",
            running: "#fb8c00"
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

        failedEl.textContent = task.failed_test || "-";
        const logs = (task.logs || "無 Log").replace(/\r\n/g, "\n");
        logsEl.textContent = logs.trim() || "無 Log";
    }

    function showError(msg) {
        errorBanner.style.display = "block";
        errorBanner.textContent = msg;
    }
});
