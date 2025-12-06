#!/bin/bash

# ==============================================================================
# 檔案位置: /home/rs/test/run_ci_task.sh
# 描述: 智慧型 CI 機器人 (具備 Re-run 與 Release 交叉驗證功能)
# ==============================================================================

# 1. 設定目標路徑
#DEFAULT_DIR="/home/rs/ci-test"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_DIR="${CI_WORK_DIR:-$(cd "$SCRIPT_DIR" && pwd)/ci-test}"
CI_TARGET_DIR="${CI_WORK_DIR:-$DEFAULT_DIR}"
CI_SCRIPT_NAME="./ci-operation.sh"

# 定義單獨測試腳本的路徑 (相對 CI_TARGET_DIR)
SINGLE_TEST_DIR="base/free5gc"
SINGLE_TEST_CMD="./test"

# 定義需要測試的環境列表
TEST_ENVS=("ulcl-ti" "ulcl-mp")  

# 初始化變數
CURRENT_ENV=""
PR_LIST=()
VERBOSE=false
FAILED_LIST_FILE=$(mktemp)

# 定義顏色
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
RESET='\033[0m'
CLEAR_LINE='\r\033[K'

# 輔助函數: 帶時間戳的 Log
log() { echo -e "[$(date +'%H:%M:%S')] $1"; }

# ==============================================================================
# 核心函數: 漂亮的測試執行器 (Pretty Test Runner)
# ==============================================================================
pretty_test_runner() {
    local spin_chars='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
    local spin_len=${#spin_chars}
    local i=0
    local current_test=""
    
    # 清空失敗列表文件
    > "$FAILED_LIST_FILE"
    
    local log_file=$(mktemp)
    local use_stdbuf=false
    if command -v stdbuf >/dev/null 2>&1; then use_stdbuf=true; fi

    set -o pipefail

    read_loop() {
        # 初始化去重緩存字串 (用於過濾重複的子測試 PASS)
        local seen_tests_str=""
        
        while true; do
            IFS= read -r -t 0.1 line
            local rc=$?
            if [ $rc -eq 0 ]; then
                process_line "$line"
            elif [ $rc -gt 128 ]; then
                process_line ""
            else
                if [ -n "$line" ]; then process_line "$line"; fi
                break
            fi
        done
    }

    if [ "$use_stdbuf" = true ]; then
        stdbuf -oL -eL "$@" 2>&1 | tee "$log_file" | read_loop
    else
        "$@" 2>&1 | tee "$log_file" | read_loop
    fi

    local status=${PIPESTATUS[0]}

    if [ $status -ne 0 ]; then
        # 注意: 這裡不再印出 Full Log，因為我們會進入 Smart Handler 處理
        # 除非這不是測試失敗而是腳本崩潰
        if [ ! -s "$FAILED_LIST_FILE" ]; then
             echo -e "\n${RED}Script Failed without capturing specific tests! Full Log:${RESET}"
             cat "$log_file"
             rm -f "$log_file"
             return $status
        fi
    fi

    rm -f "$log_file"
    return $status
}

process_line() {
    local line="$1"
    if [ -z "$line" ]; then
        if [ -n "$current_test" ]; then
            i=$(( (i+1) % spin_len ))
            printf "${CLEAR_LINE} ${YELLOW}${spin_chars:$i:1}${RESET} Running: ${current_test}"
        fi
        return
    fi

    if [[ "$line" =~ ^Test[a-zA-Z0-9_]+$ ]]; then
        current_test="$line"
        i=0
        printf "${CLEAR_LINE} ${YELLOW}${spin_chars:$i:1}${RESET} Running: ${current_test}"
    
    elif [[ "$line" =~ PASS:[[:space:]]*(Test[a-zA-Z0-9_]+) ]]; then
        local test_name="${BASH_REMATCH[1]}"
        
        # 檢查是否已經顯示過這個測試的 PASS (字串包含檢查)
        if [[ "$seen_tests_str" != *" $test_name "* ]]; then
            printf "${CLEAR_LINE} ${GREEN}✔ PASS${RESET}: %s\n" "$test_name"
            # 將測試名稱加入緩存，前後加空格以確保精確匹配
            seen_tests_str+=" $test_name "
        fi
        current_test=""

    elif [[ "$line" =~ FAIL:[[:space:]]*(Test[a-zA-Z0-9_]+) ]]; then
        local test_name="${BASH_REMATCH[1]}"
        printf "${CLEAR_LINE} ${RED}✘ FAIL${RESET}: %s\n" "$test_name"
        echo "$test_name" >> "$FAILED_LIST_FILE"
        current_test=""
    fi
}

# ==============================================================================
# 🤖 智慧型失敗處理器 (Smart Failure Handler)
# ==============================================================================
smart_failure_handler() {
    local step_name="$1"
    
    if [ ! -s "$FAILED_LIST_FILE" ]; then
        log "${RED}測試失敗，但未能解析出具體的 TestName。無法進行自動修復。${RESET}"
        return 1
    fi

    # 讀取失敗列表
    local failed_tests=()
    mapfile -t failed_tests < "$FAILED_LIST_FILE"
    
    echo -e "\n${CYAN}======================================================${RESET}"
    echo -e "${CYAN}🤖 機器人啟動: 偵測到 ${#failed_tests[@]} 個測試失敗${RESET}"
    echo -e "${CYAN}======================================================${RESET}"

    # ---------------------------------------------------------
    # 階段一: 單獨重跑 (Local Re-run)
    # ---------------------------------------------------------
    local real_failures=()
    
    # 切換到測試目錄 (ci-test/base/free5gc)
    local test_dir="$CI_TARGET_DIR/$SINGLE_TEST_DIR"
    if [ ! -d "$test_dir" ]; then
        log "${RED}找不到測試目錄: $test_dir${RESET}"
        return 1
    fi
    
    pushd "$test_dir" > /dev/null || return 1

    for test_name in "${failed_tests[@]}"; do
        echo -e "${MAGENTA}🔄 [Re-run] 正在單獨重跑 PR 版本: $test_name ...${RESET}"
        
        # 執行單一測試: ./test TestName
        # 使用 grep -q 靜默檢查輸出中是否有 PASS
        if $SINGLE_TEST_CMD "$test_name" 2>&1 | grep -q "PASS"; then
            echo -e "   ${GREEN}✔ 通過 (Flaky Test - 判定為不穩定但本次通過)${RESET}"
        else
            echo -e "   ${RED}✘ 依然失敗${RESET}"
            real_failures+=("$test_name")
        fi
    done
    
    popd > /dev/null || return 1

    # 如果所有重跑都通過了
    if [ ${#real_failures[@]} -eq 0 ]; then
        echo -e "${GREEN}✨ 恭喜! 所有失敗項目經重跑後均通過 (Flaky)。繼續執行後續流程。${RESET}"
        return 0
    fi

    # ---------------------------------------------------------
    # 階段二: 切換 Release 版本交叉驗證
    # ---------------------------------------------------------
    echo -e "\n${CYAN}⚠️  仍有 ${#real_failures[@]} 個測試失敗。${RESET}"
    echo -e "${CYAN}🔄 正在切換至 Release 版本進行交叉比對...${RESET}"
    
    # 還原代碼並重新編譯，刪有發PR的NF的image
    run_quiet $CI_SCRIPT_NAME pull || { log "Release Pull 失敗"; return 1; }
    for pr_entry in "${PR_LIST[@]}"; do
        IFS=':' read -r comp id <<< "$pr_entry"
        run_quiet docker rmi free5gc/${comp}-base:latest || true
        run_quiet $CI_SCRIPT_NAME build-nf "$comp" || { log "Build $comp 失敗"; return 1; }
    done

    # 再次進入測試目錄
    pushd "$test_dir" > /dev/null || return 1
    
    local pr_broken=false
    local env_broken=false
    
    for test_name in "${real_failures[@]}"; do
        echo -e "${BLUE}🔍 [Verify] 正在 Release 版本上執行: $test_name ...${RESET}"
        
        if $SINGLE_TEST_CMD "$test_name" 2>&1 | grep -q "PASS"; then
            echo -e "   ${GREEN}✔ Release 版本通過${RESET}"
            echo -e "   ${RED}🛑 結論: 這是 PR 的問題 (Regression)${RESET}"
            pr_broken=true
        else
            echo -e "   ${RED}✘ Release 版本也失敗${RESET}"
            echo -e "   ${YELLOW}⚠️  結論: 這是環境或 Release 本身的問題${RESET}"
            env_broken=true
        fi
    done
    
    popd > /dev/null || return 1
    
    echo -e "${CYAN}======================================================${RESET}"
    
    if [ "$pr_broken" = true ]; then
        log "${RED}⛔ 測試終止: 請修復您的 PR 代碼。${RESET}"
        return 2
    elif [ "$env_broken" = true ]; then
        log "${YELLOW}⛔ 測試終止: 請檢查 CI 環境或回報 Issue。${RESET}"
        return 3
    else
        return 0
    fi
}

run_test_command() {
    local step_name="$1"
    shift
    
    # 1. 執行主要的 testAll
    if [ "$VERBOSE" = true ]; then
        "$@"
        local status=$?
    else
        pretty_test_runner "$@"
        local status=$?
    fi

    # 2. 如果失敗，啟動機器人介入
    # if [ $status -ne 0 ]; then
    #     # 如果是 testAll 階段失敗，呼叫智慧處理器
    #     if [[ "$step_name" == "testAll" ]]; then
    #         smart_failure_handler "$step_name"
    #         # 注意: smart_failure_handler 回傳 0 代表修復成功/Flaky，非 0 代表真的掛了
    #         return $?
    #     else
    #         # 環境測試 (ulcl-ti) 失敗暫時直接報錯 (也可以實作類似邏輯)
    #         return $status
    #     fi
    # fi
    return $status
}

run_quiet() {
    if [ "$VERBOSE" = true ]; then "$@"; return $?; fi
    local cmd_output
    cmd_output=$("$@" 2>&1)
    local status=$?
    if [ $status -ne 0 ]; then
        echo -e "❌ 執行失敗！詳情：\n$cmd_output"
        return $status
    fi
    return 0
}

# 等待特定日誌模式，匹配後讓命令繼續在後台運行的函數
wait_for_log_then_continue_background() {
    local command="$1"
    local pattern="$2"
    local timeout=${3:-120}  # 預設 10 分鐘超時
    local start_time=$(date +%s)
    local counter=0
    local log_file=$(mktemp)
    local env=$(echo "$command" | awk -F'"' '{print $2}')
    
    log "⏳ 啟動命令並等待日誌模式: $pattern，匹配 60 次後讓命令繼續在後台運行"
    
    # 創建命名管道來捕獲命令輸出
    local fifo=$(mktemp -u)
    mkfifo "$fifo"
    
    # 在後台啟動命令，將輸出重定向到命名管道
    log "執行命令"
    eval "$command" > "$fifo" 2>&1 &
    local cmd_pid=$!
    
    # 打開命名管道進行讀取
    exec 3< "$fifo"
    
    while read -r line <&3; do
        echo "$line"  # 輸出到終端
        echo "$line" >> "$log_file"  # 保存到日誌文件
        
        # 檢查是否匹配模式
        if [[ "$line" =~ $pattern ]]; then
            counter=$((counter + 1))
            log "🎯 檢測到目標日誌模式: $pattern ($counter/35)"
            if [ $counter -eq 35 ]; then
                log "🎯 已檢測到 35 次目標日誌模式，命令將繼續在後台運行 (PID: $cmd_pid)"
                # 注意：這裡不終止命令，讓它繼續在後台運行
                # 關閉文件描述符，但命令會繼續運行
                exec 3<&-
                rm -f "$fifo"
                rm -f "$log_file"
                return 0
            fi
        fi
        
        # 檢查超時
        local current_time=$(date +%s)
        if (( current_time - start_time > timeout )); then
            #log "${RED}❌ 等待日誌模式超時 ($timeout 秒)，logs 保存到 ${env}.log${RESET}"
            #echo "Environment $env setup timed out after $timeout seconds" > "${env}.log"
            #grep "ERRO" "$log_file" >> "${env}.log"
            # 終止後台命令
            kill "$cmd_pid" 2>/dev/null || true
            exec 3<&-
            rm -f "$fifo"
            rm -f "$log_file"
            return 1
        fi
    done
    
    # 如果命令正常結束但未檢測到模式
    log "${RED}❌ 命令結束但未檢測到目標日誌模式${RESET}"
    exec 3<&-
    rm -f "$fifo"
    rm -f "$log_file"
    return 1
}

cleanup_on_failure() {
    log "${RED}流程終止，正在清理...${RESET}"
    if [ -n "$CURRENT_ENV" ]; then
        run_quiet $CI_SCRIPT_NAME down "$CURRENT_ENV" || true
    fi

    # 收集失敗日誌
    log "📋 Collecting failure logs..."
    mkdir -p "$SCRIPT_DIR/logs"
    cp -r "$CI_TARGET_DIR/base/free5gc/testing_output" "$SCRIPT_DIR/logs/" 2>/dev/null || true
    find "$CI_TARGET_DIR" -name "*.log" -exec cp {} "$SCRIPT_DIR/logs/" \; 2>/dev/null || true

    #還原代碼並重新編譯，刪有發PR的NF的image
    # run_quiet $CI_SCRIPT_NAME pull || { log "Release Pull 失敗"; return 1; }
    # for pr_entry in "${PR_LIST[@]}"; do
    #     IFS=':' read -r comp id <<< "$pr_entry"
    #     run_quiet docker rmi free5gc/${comp}-base:latest || true
    #     run_quiet $CI_SCRIPT_NAME build-nf "$comp" || { log "Build $comp 失敗"; return 1; }
    # done
    
    rm -f "$FAILED_LIST_FILE"
    exit 1
}

# 2. 解析參數
while getopts "e:p:d:nh" opt; do
    case $opt in
        e) ;;
        p) PR_LIST+=("$OPTARG") ;;
        d) CI_TARGET_DIR="$OPTARG" ;;
        n) VERBOSE=true ;; 
        *) echo "Usage: $0 -p <comp:id> [-n] [-d <dir>]"; exit 1 ;;
    esac
done

if [ ${#PR_LIST[@]} -eq 0 ]; then echo -e "⚠️  未偵測到 PR，停止執行。"; exit 0; fi

echo "=========================================="
echo "🤖 CI Smart Bot (Auto-Verification)"
echo "📂 目標目錄: $CI_TARGET_DIR"
echo "📦 待測 PR: ${PR_LIST[*]}"
echo "=========================================="

if [ ! -d "$CI_TARGET_DIR" ]; then echo -e "❌ Dir not found"; exit 1; fi
cd "$CI_TARGET_DIR" || exit 1

# ================= 準備階段 =================
log "🔄 1. Pulling source..."
run_quiet $CI_SCRIPT_NAME pull || exit 1

log "📥 2. Fetching PRs..."
for pr_entry in "${PR_LIST[@]}"; do
    IFS=':' read -r comp id <<< "$pr_entry"
    log "   -> Fetching $comp #$id"
    run_quiet $CI_SCRIPT_NAME fetch "$comp" "$id" || exit 1
done

# ================= TestAll 階段 (含機器人邏輯) =================
log "🧪 3. Normal Tests (testAll)..."

# 呼叫 run_test_command，如果它回傳 0 (成功或已修復)，才繼續
if run_test_command "testAll" $CI_SCRIPT_NAME testAll; then
    log "${GREEN}✅ Pre-build Tests Passed (or Flaky verified)!${RESET}"
else
    log "${RED}⛔ Pre-build Tests Failed (Verification confirm regression/env issue).${RESET}"
    # 這裡直接退出，不執行後面的環境測試
    # rm -f "$FAILED_LIST_FILE"
    
    # # 收集日誌
    # log "📋 Collecting logs..."
    # mkdir -p "$SCRIPT_DIR/logs"
    # cp -r "$CI_TARGET_DIR/base/free5gc/testing_output" "$SCRIPT_DIR/logs/" 2>/dev/null || true
    # find "$CI_TARGET_DIR" -name "*.log" -exec cp {} "$SCRIPT_DIR/logs/" \; 2>/dev/null || true

    # exit 1
fi

log "🏗️ 5. Building..."
#run_quiet $CI_SCRIPT_NAME build || { log "Build 失敗"; exit 1; }
# build有發PR的NF的image
# for pr_entry in "${PR_LIST[@]}"; do
#     IFS=':' read -r comp id <<< "$pr_entry"
#     run_quiet docker rmi free5gc/${comp}-base:latest || true
#     run_quiet $CI_SCRIPT_NAME build-nf "$comp" || { log "Build $comp 失敗"; return 1; }
# done

# ================= 循環測試階段 =================
log "🚀 Starting Test Cycles..."

# for ENV in "${TEST_ENVS[@]}"; do
#     CURRENT_ENV="$ENV"
    
#     echo "------------------------------------------------"
#     log "▶️  Testing Environment: $CURRENT_ENV"
#     log "🔌 Starting ($CURRENT_ENV)..."
#     #run_quiet $CI_SCRIPT_NAME up "$CURRENT_ENV" || cleanup_on_failure
#     # 等待 60 次 handleHeartbeatRequest 日誌，匹配後讓命令繼續在後台運行
#     wait_for_log_then_continue_background "$CI_SCRIPT_NAME up \"$CURRENT_ENV\"" "handleHeartbeatRequest" || cleanup_on_failure
    
#     log "⚡ Running tests ($CURRENT_ENV)..."
    
#     if run_test_command "$ENV" $CI_SCRIPT_NAME test "$ENV"; then
#         log "${GREEN}✅ All Tests Passed ($CURRENT_ENV)!${RESET}"
#     else
#         log "${RED}❌ Tests Failed ($CURRENT_ENV)${RESET}"
#         cleanup_on_failure
#     fi

#     log "🛑 Shutting down ($CURRENT_ENV)..."
#     run_quiet $CI_SCRIPT_NAME down "$CURRENT_ENV" || cleanup_on_failure
#     CURRENT_ENV=""
# done

# 還原代碼並重新編譯，刪有發PR的NF的image
# run_quiet $CI_SCRIPT_NAME pull || { log "Release Pull 失敗"; return 1; }
# for pr_entry in "${PR_LIST[@]}"; do
#     IFS=':' read -r comp id <<< "$pr_entry"
#     run_quiet docker rmi free5gc/${comp}-base:latest || true
#     run_quiet $CI_SCRIPT_NAME build-nf "$comp" || { log "Build $comp 失敗"; return 1; }
# done

# 收集日誌
log "📋 Collecting logs..."
mkdir -p "$SCRIPT_DIR/logs"
cp -r "$CI_TARGET_DIR/base/free5gc/testing_output" "$SCRIPT_DIR/logs/" 2>/dev/null || true
find "$CI_TARGET_DIR" -name "*.log" -not -path "*/testing_output/*" -exec cp {} "$SCRIPT_DIR/logs/" \; 2>/dev/null || true

log "🎉 All Tasks Completed!"
rm -f "$FAILED_LIST_FILE"