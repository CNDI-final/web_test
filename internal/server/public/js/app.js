document.addEventListener("DOMContentLoaded", () => {
    // === DOM 元素取得 ===
    const ghBtn = document.getElementById("gh-btn");
    const ghOwner = document.getElementById("gh-owner");
    const ghRepo = document.getElementById("gh-repo");

    const prSelect = document.getElementById("pr-select");
    const actionSelect = document.getElementById("action-select");
    const runPrBtn = document.getElementById("run-pr-btn");
    const runMsg = document.getElementById("run-msg");
    
    const addParamBtn = document.getElementById("add-param-btn");
    const paramsContainer = document.getElementById("params-container");

    const queueBody = document.getElementById("queue-table-body");
    const runningList = document.getElementById("running-list");
    const historyList = document.getElementById("history-list");

    // ==========================================
    // 1. GitHub 抓取按鈕
    // ==========================================
    if (ghBtn) {
        ghBtn.addEventListener("click", async () => {
            const owner = ghOwner.value.trim();
            const repo = ghRepo.value.trim();
            if (!owner || !repo) return alert("請輸入 Owner 和 Repo");

            try {
                await fetch("/api/queue/add_github", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ owner, repo })
                });
                loadAll(); // 觸發刷新
            } catch (e) { console.error(e); }
        });
    }

    // ==========================================
    // 2. 動態參數新增按鈕
    // ==========================================
    if (addParamBtn) {
        addParamBtn.addEventListener("click", () => {
            const div = document.createElement("div");
            div.className = "param-row";

            // 產生 arg0 ~ arg5 的下拉選單
            let opts = "";
            for(let i=0; i<=5; i++) opts += `<option value="arg${i}">arg${i}</option>`;

            div.innerHTML = `
                <select class="param-key" style="width:80px;">${opts}</select>
                <input type="text" class="param-val" placeholder="Value" style="width:150px;">
                <button class="btn-del" style="background:#999; color:white; border:none; cursor:pointer;">✕</button>
            `;
            
            div.querySelector(".btn-del").addEventListener("click", () => div.remove());
            paramsContainer.appendChild(div);
        });
    }

    // ==========================================
    // 3. PR 執行按鈕 (收集參數並發送)
    // ==========================================
    if (runPrBtn) {
        runPrBtn.addEventListener("click", async () => {
            const selectedOption = prSelect.options[prSelect.selectedIndex];
            if (!selectedOption || !selectedOption.value) return alert("請選擇一個 PR");

            // 收集動態參數
            const paramsMap = {};
            document.querySelectorAll(".param-row").forEach(row => {
                const key = row.querySelector(".param-key").value;
                const val = row.querySelector(".param-val").value.trim();
                if (val) paramsMap[key] = val;
            });

            runMsg.innerText = "發送中...";
            try {
                const res = await fetch("/api/run-pr", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        pr_number: parseInt(selectedOption.value),
                        pr_title: selectedOption.dataset.title,
                        action: actionSelect.value,
                        params: paramsMap
                    })
                });
                const data = await res.json();
                runMsg.innerText = data.reply;
                loadAll(); // 刷新佇列
            } catch (e) {
                runMsg.innerText = "錯誤: " + e;
            }
        });
    }

    // ==========================================
    // 4. 刪除佇列任務 (Event Delegation)
    // ==========================================
    if (queueBody) {
        queueBody.addEventListener("click", async (e) => {
            if (e.target.classList.contains("btn-del")) {
                const id = e.target.dataset.id;
                if(confirm(`確定要移除任務 ID ${id} 嗎?`)) {
                    await fetch(`/api/queue/delete/${id}`, { method: "DELETE" });
                    loadAll();
                }
            }
        });
    }

    // ==========================================
    // 5. 定期更新函式庫
    // ==========================================

    // 更新 PR 下拉選單
    async function updatePRList() {
        try {
            const res = await fetch("/api/prs");
            const prs = await res.json();
            
            if (!prs || prs.length === 0) return;

            const currentVal = prSelect.value;
            prSelect.innerHTML = "";
            prs.forEach(pr => {
                const opt = document.createElement("option");
                opt.value = pr.number;
                let displayTitle = pr.title.length > 40 ? pr.title.substring(0, 40) + "..." : pr.title;
                opt.text = `#${pr.number}: ${displayTitle}`;
                opt.dataset.title = pr.title;
                prSelect.appendChild(opt);
            });
            if (currentVal) prSelect.value = currentVal;
        } catch (e) {}
    }

    // 更新執行中任務 (進度條)
    async function loadRunning() {
        try {
            const res = await fetch("/api/running");
            const tasks = await res.json();
            
            runningList.innerHTML = "";
            if (!tasks || tasks.length === 0) {
                runningList.innerHTML = "<tr><td colspan='3' style='color:#999; text-align:center;'>無任務執行中</td></tr>";
                return;
            }

            tasks.forEach(t => {
                runningList.innerHTML += `
                    <tr>
                        <td><small>${t.task_name}</small></td>
                        <td>
                            <div style="background:#eee; width:100%; height:8px; border-radius:4px; overflow:hidden;">
                                <div style="background:#2196f3; width:${t.percent}%; height:100%; transition: width 0.5s;"></div>
                            </div>
                        </td>
                        <td style="color:red; font-weight:bold;">${t.remaining}s</td>
                    </tr>`;
            });
        } catch (e) {}
    }

    // 更新排隊列表
    async function loadQueue() {
        try {
            const res = await fetch("/api/queue/list");
            const tasks = await res.json();
            
            queueBody.innerHTML = "";
            if (!tasks || tasks.length === 0) {
                queueBody.innerHTML = "<tr><td colspan='3' style='color:#999; text-align:center;'>目前空閒</td></tr>";
                return;
            }
            tasks.forEach(t => {
                queueBody.innerHTML += `
                    <tr>
                        <td>${t.task_id}</td>
                        <td><small>${t.task_name}</small></td>
                        <td><button class="btn-del" data-id="${t.task_id}">移除</button></td>
                    </tr>`;
            });
        } catch (e) {}
    }

    // 更新歷史紀錄
    async function loadHistory() {
        try {
            const res = await fetch("/api/history");
            const records = await res.json();
            
            historyList.innerHTML = "";
            if (!records) return;

            records.forEach(r => {
                historyList.innerHTML += `
                    <tr>
                        <td>${r.time}</td>
                        <td>${r.task_name}</td>
                        <td style='color:green'>${r.result}</td>
                    </tr>`;
            });
        } catch (e) {}
    }

    function loadAll() {
        loadQueue();
        loadRunning();
        loadHistory();
    }

    // 啟動定時器
    setInterval(loadAll, 1000);      // 狀態類每秒更新
    setInterval(updatePRList, 3000); // 選單類 3 秒更新
    loadAll(); 
});