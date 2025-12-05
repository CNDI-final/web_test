document.addEventListener("DOMContentLoaded", () => {
    // === DOM 元素取得 ===
    const nfSelect = document.getElementById("nf-select");
    const prSelect = document.getElementById("pr-select");
    const actionSelect = document.getElementById("action-select");
    
    const addToListBtn = document.getElementById("add-to-list-btn");
    const selectedTasksBody = document.getElementById("selected-tasks-body");
    const runAllBtn = document.getElementById("run-all-btn");
    const runMsg = document.getElementById("run-msg");

    const queueBody = document.getElementById("queue-table-body");
    const runningList = document.getElementById("running-list");
    const historyList = document.getElementById("history-list");

    // 本地暫存的待執行任務列表
    let selectedTasks = [];

    // ==========================================
    // 1. NF 選擇變更 -> 觸發 GitHub 抓取
    // ==========================================
    if (nfSelect) {
        nfSelect.addEventListener("change", async () => {
            const repo = nfSelect.value;
            const owner = "free5gc";
            
            prSelect.innerHTML = '<option>載入中...</option>';
            
            try {
                // 呼叫後端抓取 (Worker 2)
                await fetch("/api/queue/add_github", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ owner, repo })
                });
                loadAll(); // 觸發刷新
                // 簡單的輪詢等待結果 (或是依賴原本的 updatePRList 定時更新)
                // 這裡為了體驗更好，稍微延遲後立即觸發一次更新
                setTimeout(updatePRList, 500);
                setTimeout(updatePRList, 1500);
                // setTimeout(updatePRList, 3000);
            } catch (e) { console.error(e); }
        });
    }

    // ==========================================
    // 2. 添加到下區塊列表
    // ==========================================
    if (addToListBtn) {
        addToListBtn.addEventListener("click", () => {
            const nf = nfSelect.value;
            const prVal = prSelect.value;
            const action = actionSelect.value;
            
            if (!nf) return alert("請選擇 NF");
            if (!prVal) return alert("請選擇 PR");

            const selectedOption = prSelect.options[prSelect.selectedIndex];
            const prTitle = selectedOption.dataset.title || "";
            const prNumber = parseInt(prVal);

            const task = {
                id: Date.now(), // 暫時 ID
                nf: nf,
                prNumber: prNumber,
                prTitle: prTitle,
                action: action
            };

            selectedTasks.push(task);
            renderSelectedTasks();
        });
    }

    function renderSelectedTasks() {
        selectedTasksBody.innerHTML = "";
        selectedTasks.forEach((task, index) => {
            const tr = document.createElement("tr");
            tr.innerHTML = `
                <td>${task.nf}</td>
                <td>#${task.prNumber}: ${task.prTitle.substring(0, 20)}...</td>
                <td>${task.action}</td>
                <td><button class="btn-del" data-local-id="${task.id}">刪除</button></td>
            `;
            selectedTasksBody.appendChild(tr);
        });
    }

    // 監聽下區塊的刪除按鈕
    if (selectedTasksBody) {
        selectedTasksBody.addEventListener("click", (e) => {
            if (e.target.classList.contains("btn-del")) {
                const id = parseInt(e.target.dataset.localId);
                selectedTasks = selectedTasks.filter(t => t.id !== id);
                renderSelectedTasks();
            }
        });
    }

    // ==========================================
    // 3. 執行所有任務
    // ==========================================
    if (runAllBtn) {
        runAllBtn.addEventListener("click", async () => {
            if (selectedTasks.length === 0) return alert("沒有待執行的任務");

            runMsg.innerText = "發送中...";
            runAllBtn.disabled = true;

            try {
                for (const task of selectedTasks) {
                    const params = {
                        pr_number: String(task.prNumber),
                        pr_title: `[${task.nf}] ${task.prTitle}`,
                        action: task.action
                    };
                    await fetch("/api/run-pr", {
                        method: "POST",
                        headers: { "Content-Type": "application/json" },
                        body: JSON.stringify({ params })
                    });
                }
                
                runMsg.innerText = `已發送 ${selectedTasks.length} 個任務`;
                selectedTasks = [];
                renderSelectedTasks();
                loadAll(); // 刷新佇列
            } catch (e) {
                runMsg.innerText = "錯誤: " + e;
            } finally {
                runAllBtn.disabled = false;
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
        // 如果使用者正在操作選單，暫停更新以免干擾
        if (document.activeElement === prSelect) return;

        try {
            // 如果選單顯示「載入中...」，則跳過更新，避免顯示上次的資料
            if (prSelect.options.length === 1 && prSelect.options[0].text === "-- 請先選擇 NF --") {
                return;
            }
            
            const res = await fetch("/api/prs");
            const prs = await res.json();
            
            // 清空選單
            const currentVal = prSelect.value;
            prSelect.innerHTML = "";

            if (!prs || prs.length === 0) {
                const opt = document.createElement("option");
                opt.text = "-- 無 PR --";
                prSelect.appendChild(opt);
                return;
            }

            // 顯示前 5 個 PR
            const displayLimit = 5;
            const initialPRs = prs.slice(0, displayLimit);
            
            initialPRs.forEach(pr => {
                const opt = document.createElement("option");
                opt.value = pr.number;
                let displayTitle = pr.title.length > 40 ? pr.title.substring(0, 40) + "..." : pr.title;
                opt.text = `#${pr.number}: ${displayTitle}`;
                opt.dataset.title = pr.title;
                prSelect.appendChild(opt);
            });

            // 如果超過 5 個，加入 "..." 選項
            if (prs.length > displayLimit) {
                const moreOpt = document.createElement("option");
                moreOpt.value = "LOAD_MORE";
                moreOpt.text = "... (載入更多)";
                prSelect.appendChild(moreOpt);
            }
            
            // 監聽選擇變更，如果選到 "..." 則載入剩餘 PR
            prSelect.onchange = function() {
                if (this.value === "LOAD_MORE") {
                    // 移除 "..." 選項
                    const moreOpt = this.querySelector('option[value="LOAD_MORE"]');
                    if (moreOpt) moreOpt.remove();

                    // 在最前面插入預設選項
                    const defaultOpt = document.createElement("option");
                    defaultOpt.value = "";
                    defaultOpt.text = "-- 選擇 PR --";
                    defaultOpt.disabled = true;
                    defaultOpt.selected = true;
                    prSelect.insertBefore(defaultOpt, prSelect.firstChild);

                    // 加入剩餘的 PR
                    const remainingPRs = prs.slice(displayLimit);
                    remainingPRs.forEach(pr => {
                        const opt = document.createElement("option");
                        opt.value = pr.number;
                        let displayTitle = pr.title.length > 40 ? pr.title.substring(0, 40) + "..." : pr.title;
                        opt.text = `#${pr.number}: ${displayTitle}`;
                        opt.dataset.title = pr.title;
                        prSelect.appendChild(opt);
                    });
                    
                    // 移除這個特殊的 onchange 處理器，恢復正常操作
                    this.onchange = null;
                }
            };
            
            // 嘗試保留原本的選擇
            if (currentVal && currentVal !== "LOAD_MORE") {
                const exists = Array.from(prSelect.options).some(o => o.value === currentVal);
                if (exists) prSelect.value = currentVal;
            }
        } catch (e) {
            console.error("更新 PR 列表失敗:", e);
            prSelect.innerHTML = "<option>更新失敗</option>";
        }
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
                const resultText = (r.result && r.result.startsWith("完成")) ? "完成" : r.result;
                historyList.innerHTML += `
                    <tr>
                        <td>${r.time}</td>
                        <td>${r.task_name}</td>
                        <td style='color:green'>${resultText}</td>
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
    setInterval(updatePRList, 2000); // 選單類 2 秒更新
    loadAll(); 
});