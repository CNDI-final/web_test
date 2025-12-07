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
CI_SCRIPT_NAME="$CI_TARGET_DIR/ci-operation.sh"

# 定義單獨測試腳本的路徑 (相對 CI_TARGET_DIR)
SINGLE_TEST_DIR="base/free5gc"
SINGLE_TEST_CMD="./test.sh"

# 定義需要測試的環境列表
TEST_ENVS=("ulcl-ti" "ulcl-mp")
TEST_POOL="TestRegistration|TestGUTIRegistration|TestServiceRequest|TestXnHandover|TestN2Handover|TestDeregistration|TestPDUSessionReleaseRequest|TestPaging|TestNon3GPP|TestReSynchronization|TestDuplicateRegistration|TestEAPAKAPrimeAuthentication|TestMultiAmfRegistration|TestNasReroute|TestTngf|TestDC|TestDynamicDC|TestXnDCHandover"
# 初始化變數
CURRENT_ENV=""
PR_LIST=()
VERBOSE=false
REGRESS=true
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
    
    elif [[ "$line" =~ ^[[:space:]]*---[[:space:]]+FAIL:[[:space:]]+(.+) ]]; then
        local sub_test="${BASH_REMATCH[1]}"
        local sub_name="${sub_test#*/}"
        printf "${CLEAR_LINE} ${RED}FAIL:${RESET} ${sub_name}\n"
    
    elif [[ "$line" =~ ^[[:space:]]*---[[:space:]]+PASS:[[:space:]]+(.+) ]]; then
        local sub_test="${BASH_REMATCH[1]}"
        local sub_name="${sub_test#*/}"
        printf "${CLEAR_LINE} ${GREEN}PASS:${RESET} ${sub_name}\n"
    
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
    # 切換到測試目錄 (ci-test/base/free5gc)
    local test_dir="$CI_TARGET_DIR/$SINGLE_TEST_DIR"
    if [ ! -d "$test_dir" ]; then
        log "${RED}找不到測試目錄: $test_dir${RESET}"
        return 1
    fi

    for phase in 1 2; do
        # 讀取 JSON 內容
        json_content=$(cat "$SCRIPT_DIR/logs/failures.json")
        array_part=$(echo "$json_content" | sed 's/.*"failed_tests": \[\([^]]*\)\].*/\1/')
        if [ -z "$array_part" ]; then
            failed_list=()
        else
            IFS=',' read -ra failed_list <<< "$(echo "$array_part" | tr -d '"' | tr -d ' ')"
        fi
        if [ $phase -eq 1 ]; then
                echo -e "\n${CYAN}======================================================${RESET}"
                echo -e "${CYAN}🤖 機器人啟動: 偵測到 ${#failed_list[@]} 個測試失敗${RESET}"
                echo -e "${CYAN}======================================================${RESET}"
            # ---------------------------------------------------------
            # 階段一: 單獨重跑 (Local Re-run)
            # ---------------------------------------------------------
        else
            # ---------------------------------------------------------
            # 階段二: 切換 Release 版本交叉驗證
            # ---------------------------------------------------------
            echo -e "\n${CYAN}⚠️  仍有 ${#failed_list[@]} 個測試失敗。${RESET}"
            echo -e "${CYAN}🔄 正在切換至 Release 版本進行交叉比對...${RESET}"
            run_quiet $CI_SCRIPT_NAME pull || exit 5
        fi
        pushd "$test_dir" > /dev/null || exit 6
        make all
        ./force_kill.sh
        mkdir -p testing_output
        for test_name in "${failed_list[@]}"; do
            test_name="${test_name%.log}"
            
            echo "$test_name"
            echo "    Output saved to testing_output/$test_name.log"
            exec $SINGLE_TEST_CMD "$test_name" &> "$test_dir/testing_output/$test_name.log" &
            wait
            if [[ "$test_name" == "TestTngf" || "$test_name" == "TestNon3GPP" ]]; then
                sudo killall -9 n3iwf tngf 2>/dev/null
                sleep 2
            fi
            STATUS=$(grep -a -E "\-\-\-.*:" "$test_dir/testing_output/$test_name.log")
            if [ ! -z "$STATUS" ]; then
                echo "$STATUS" | while read -r a; do echo "    ${a:4}"; done
            else
                echo "    Failed"
                echo "exit status 1" >> "$test_dir/testing_output/$test_name.log"
            fi
            echo
        done
        if [ $phase -eq 1 ]; then
            getlog
            scan_logs "testall"
        fi
        scan_logs "testall" "$test_dir"
        local status=$?
        if [ $phase -eq 1 ]; then
            # 如果所有重跑都通過了
            if [ $status -eq 0 ]; then
                echo -e "${GREEN}✨ 恭喜! 所有失敗項目經重跑後均通過 (Flaky)。繼續執行後續流程。${RESET}"
                popd > /dev/null || exit 6
                return 0
            fi
        else
            echo -e "${CYAN}======================================================${RESET}"
            if [ $status -ne 0 ]; then
                log "${YELLOW}⛔ 測試終止: 請檢查 CI 環境或回報 Issue。${RESET}"
                exit 2
            else
                log "${RED}⛔ 測試終止: 請修復您的 PR。${RESET}"
                exit 3
            fi
        fi
        popd > /dev/null || exit 6
    done
}

smart_failure_handler_ulcl() {
    local env="$1"
    
    for phase in 1 2; do
        if [ $phase -eq 1 ]; then
            echo -e "\n${CYAN}======================================================${RESET}"
            echo -e "${CYAN}🤖 機器人啟動: $env 測試失敗，重試中${RESET}"
            echo -e "${CYAN}======================================================${RESET}"
            # 階段一: 本地重試
            CURRENT_ENV="$env"
            
            echo "------------------------------------------------"
            log "▶️  Testing Environment: $CURRENT_ENV"
            log "🔌 Starting ($CURRENT_ENV)..."
            # 等待 60 次 handleHeartbeatRequest 日誌，匹配後讓命令繼續在後台運行
            wait_for_log_then_continue_background "$CI_SCRIPT_NAME up \"$CURRENT_ENV\"" "handleHeartbeatRequest" || cleanup_on_failure
            
            log "⚡ Running tests ($CURRENT_ENV)..."
            if [ "$VERBOSE" = true ]; then
                $CI_SCRIPT_NAME test "$CURRENT_ENV"
            else
                pretty_test_runner $CI_SCRIPT_NAME test "$CURRENT_ENV"
            fi
            log "🛑 Shutting down ($CURRENT_ENV)..."
            run_quiet $CI_SCRIPT_NAME down "$CURRENT_ENV" || cleanup_on_failure
            getlog
            scan_logs "$CURRENT_ENV"
            local status=$?
            if [ $status -eq 0 ] ; then
                log "${GREEN}✨ 恭喜! $env 環境測試經重試後通過。繼續執行後續流程。${RESET}"
                CURRENT_ENV=""
                return 0
            else
                log "${RED}❌[$CURRENT_ENV]Some Tests Failed ${RESET}"
            fi
            CURRENT_ENV=""
        else
            # 階段二: 切換 Release 版本交叉驗證
            echo -e "\n${CYAN}⚠️  仍有環境測試失敗。${RESET}"
            echo -e "${CYAN}🔄 正在切換至 Release 版本進行交叉比對...${RESET}"
            restore_and_build
            CURRENT_ENV="$env"
            
            echo "------------------------------------------------"
            log "▶️  Testing Environment: $CURRENT_ENV"
            log "🔌 Starting ($CURRENT_ENV)..."
            # 等待 60 次 handleHeartbeatRequest 日誌，匹配後讓命令繼續在後台運行
            wait_for_log_then_continue_background "$CI_SCRIPT_NAME up \"$CURRENT_ENV\"" "handleHeartbeatRequest" || cleanup_on_failure
            
            log "⚡ Running tests ($CURRENT_ENV)..."
            if [ "$VERBOSE" = true ]; then
                $CI_SCRIPT_NAME test "$CURRENT_ENV"
            else
                pretty_test_runner $CI_SCRIPT_NAME test "$CURRENT_ENV"
            fi
            log "🛑 Shutting down ($CURRENT_ENV)..."
            run_quiet $CI_SCRIPT_NAME down "$CURRENT_ENV" || cleanup_on_failure       
            scan_logs "$CURRENT_ENV" "$CI_TARGET_DIR"
            CURRENT_ENV="" 
            local status=$?
            if [ $status -eq 0 ]; then
                log "${RED}⛔ 測試終止: 請修復您的 PR。${RESET}"
                return 3
            else
                log "${RED}⛔ 測試終止: 請檢查 CI 環境或回報 Issue。${RESET}"
                return 2
            fi
        fi
    done
}

run_test_command() {
    local step_name="$1"
    shift
    
    #1. 執行主要的 testAll
    if [[ "$step_name" == "testAll" ]]; then
        test_all
    else
        if [ "$VERBOSE" = true ]; then
            "$@"
        else
            pretty_test_runner "$@"
        fi
    fi
    if [ $REGRESS = true ]; then
        getlog
        scan_logs "$step_name"
        local status=$?
        if [ $status -ne 0 ] ; then
            # 如果是 testAll 階段失敗，呼叫智慧處理器
            if [[ "$step_name" == "testAll" ]]; then
                # 注意: smart_failure_handler 回傳 0 代表修復成功/Flaky，非 0 代表真的掛了
                smart_failure_handler "$step_name"
                return $?
            else
                # 環境測試 (ulcl-ti)
                smart_failure_handler_ulcl "$step_name"
                return $?
            fi
        fi
    fi
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
    local spin_chars='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
    local spin_len=${#spin_chars}
    local i=0
    
    log "⏳ 啟動命令並等待日誌模式: $pattern，匹配 35 次後讓命令繼續在後台運行"
    
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
        if [ "$VERBOSE" = true ]; then
            echo "$line"
        fi
        echo "$line" >> "$log_file"  # 保存到日誌文件
        
        # 檢查是否匹配模式
        if [[ "$line" =~ $pattern ]]; then
            counter=$((counter + 1))
            if [ "$VERBOSE" = false ]; then
                i=$(( (i+1) % spin_len ))
                printf "\r\033[K${YELLOW}${spin_chars:$i:1} 檢測到目標日誌模式: $pattern ($counter/15)${RESET}"
            else
                log "🎯 檢測到目標日誌模式: $pattern ($counter/15)${RESET}"
            fi
            if [ $counter -eq 15 ]; then
                if [ "$VERBOSE" = false ]; then
                    printf "\n"
                fi
                log "🎯 已檢測到 15 次目標日誌模式，命令將繼續在後台運行 (PID: $cmd_pid)"
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
            # 終止後台命令
            kill "$cmd_pid" 2>/dev/null || true
            exec 3<&-
            rm -f "$fifo"
            rm -f "$log_file"
            cleanup_on_failure
            return 4
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

    log "📋 Collecting logs..."
    mkdir -p "$SCRIPT_DIR/logs"
    find "$CI_TARGET_DIR" -type f -iname "*.log" -exec cp {} "$SCRIPT_DIR/logs/" \; 2>/dev/null || true
    getlog
    restore_and_build
    
    rm -f "$FAILED_LIST_FILE"
    exit 1
}

test_all() {
    local test_dir="$CI_TARGET_DIR/$SINGLE_TEST_DIR"
    pushd "$test_dir" > /dev/null || return 1
    run_quiet make all
    run_quiet ./force_kill.sh
    echo "Running All Tests"
    mkdir -p testing_output
    IFS='|' read -ra ADDR <<< "$TEST_POOL"
    local spin_chars='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
    local spin_len=${#spin_chars}
    local i=0
    for test_name in "${ADDR[@]}"; do
        exec $SINGLE_TEST_CMD "$test_name" &> "$test_dir/testing_output/$test_name.log" &
        local PID=$!
        while kill -0 $PID 2>/dev/null; do
            i=$(( (i+1) % spin_len ))
            printf "${CLEAR_LINE} ${YELLOW}${spin_chars:$i:1}${RESET} Running: ${test_name}"
            sleep 0.1
        done
        wait $PID
        printf "${CLEAR_LINE}"
        if [[ "$test_name" == "TestTngf" || "$test_name" == "TestNon3GPP" ]]; then
            sudo killall -9 n3iwf tngf 2>/dev/null
            sleep 2
        fi
        STATUS=$(grep -a -E "\-\-\-.*:" "$test_dir/testing_output/$test_name.log")
        if [ ! -z "$STATUS" ]; then
            echo "$STATUS" | while read -r a; do 
                if [[ "$a" =~ PASS ]]; then
                    echo -e "    ${GREEN}✔ ${a:4}${RESET}"
                elif [[ "$a" =~ FAIL ]]; then
                    echo -e "    ${RED}✘ ${a:4}${RESET}"
                else
                    echo "    ${a:4}"
                fi
            done
        else
            echo -e "${RED}✘ FAIL:"$test_name"${RESET}"
            echo "exit status 1" >> "$test_dir/testing_output/$test_name.log"
        fi
    done
    popd > /dev/null || return 1
}

ulcl_test_cycle() {
    CURRENT_ENV="$1"
    
    echo "------------------------------------------------"
    log "▶️  Testing Environment: $CURRENT_ENV"
    log "🔌 Starting ($CURRENT_ENV)..."
    # 等待 60 次 handleHeartbeatRequest 日誌，匹配後讓命令繼續在後台運行
    wait_for_log_then_continue_background "$CI_SCRIPT_NAME up \"$CURRENT_ENV\"" "handleHeartbeatRequest" || cleanup_on_failure
    
    log "⚡ Running tests ($CURRENT_ENV)..."
    run_test_command "$CURRENT_ENV" $CI_SCRIPT_NAME test "$CURRENT_ENV"
    local status=$?
    if [ $status -eq 2 ] || [ $status -eq 3 ] ; then
        exit $status
    fi
    getlog
    if scan_logs "$CURRENT_ENV"; then
        log "${GREEN}✅ [$CURRENT_ENV] Tests Passed!${RESET}"
    else
        log "${RED}⛔ [$CURRENT_ENV] Tests Failed.${RESET}"
    fi

    log "🛑 Shutting down ($CURRENT_ENV)..."
    run_quiet $CI_SCRIPT_NAME down "$CURRENT_ENV" || cleanup_on_failure
    CURRENT_ENV=""
    return $status
}

# 還原代碼並重新編譯，刪有發PR的NF的image
restore_and_build() {
    run_quiet $CI_SCRIPT_NAME pull || { log "Release Pull 失敗"; exit 5; }
    for pr_entry in "${PR_LIST[@]}"; do
        IFS=':' read -r comp id <<< "$pr_entry"
        run_quiet $CI_SCRIPT_NAME build-nf "$comp" || { log "Build $comp 失敗"; exit 4; }
    done
}

getlog() {
    log "📋 Collecting logs..."
    mkdir -p "$SCRIPT_DIR/logs"
    find "$CI_TARGET_DIR" -type f -iname "*.log" -exec cp {} "$SCRIPT_DIR/logs/" \; 2>/dev/null || true
}
# 在 logs 裡掃描是否有 'exit status 1' 的測試紀錄，並輸出 JSON
scan_logs() {
    local filter_type="$1"
    local log_dir="${2:-$SCRIPT_DIR/logs}"
    log "🔎 Scanning $log_dir for files containing 'exit status 1'..."
    
    if [ "$filter_type" = "" ]; then
        filter_type="All"
    fi
    # 根據參數過濾文件名
    local filter_cmd=""
    if [ "$filter_type" = "ulcl" ]; then
        filter_cmd="grep ULCL"
    elif [ "$filter_type" = "ulcl-mp" ]; then
        filter_cmd="grep ULCLM"
    elif [ "$filter_type" = "ulcl-ti" ]; then
        filter_cmd="grep ULCLT"
    elif [ "$filter_type" = "testall" ]; then
        filter_cmd="grep -v ULCL"
    fi
    
    # 抓出所有包含 'exit status 1' 的 .log 檔案名稱，去重
    local cmd="find \"$log_dir\" -type f -name \"*.log\" -exec grep -l 'exit status 1' {} \; | xargs -n1 basename 2>/dev/null | sort -u"
    if [ -n "$filter_cmd" ]; then
        cmd="$cmd | $filter_cmd"
    fi
    
    mapfile -t failed_tests < <(eval "$cmd")
    
    #紀錄測試失敗
    json_file="$log_dir/failures.json"
    if [ ${#failed_tests[@]} -gt 0 ]; then
        printf '{"failed_tests": [' > "$json_file"
        for i in "${!failed_tests[@]}"; do
            name="${failed_tests[$i]}"
            esc=$(printf '%s' "$name" | sed 's/"/\\"/g')
            if [ "$i" -ne 0 ]; then printf ',' >> "$json_file"; fi
            printf '"%s"' "$esc" >> "$json_file"
        done
        printf ']}' >> "$json_file"
        log "${RED}❌ [$filter_type] ${#failed_tests[@]} failed tests${RESET}(saved to $json_file)"
        return 1
    else
        printf '{"failed_tests": []}\n' > "$json_file"
        log "${GREEN}✅ [$filter_type] All tests passed ${RESET}"
        return 0
    fi
}

# 2. 解析參數
while getopts "e:p:d:nh:r" opt; do
    case $opt in
        e) ;;
        p) PR_LIST+=("$OPTARG") ;;
        d) CI_TARGET_DIR="$OPTARG" ;;
        n) VERBOSE=true ;; 
        r) REGRESS=true ;;
        *) echo "Usage: $0 -p <comp:id> [-n] [-d <dir>]"; exit 1 ;;
    esac
done

# if [ ${#PR_LIST[@]} -eq 0 ]; then echo -e "⚠️  未偵測到 PR，停止執行。"; exit 0; fi

echo "=========================================="
echo "🤖 CI Smart Bot (Auto-Verification)"
echo "📂 目標目錄: $CI_TARGET_DIR"
echo "📦 待測 PR: ${PR_LIST[*]}"
echo "=========================================="

if [ ! -d "$CI_TARGET_DIR" ]; then echo -e "❌ Dir not found"; exit 1; fi
cd "$CI_TARGET_DIR" || exit 1

# ================= 準備階段 =================
log "🔄 1. Pulling source..."
run_quiet $CI_SCRIPT_NAME pull || exit 5

log "📥 2. Fetching PRs..."
for pr_entry in "${PR_LIST[@]}"; do
    IFS=':' read -r comp id <<< "$pr_entry"
    log "   -> Fetching $comp #$id"
    run_quiet $CI_SCRIPT_NAME fetch "$comp" "$id" || exit 1
done

# ================= TestAll 階段 (含機器人邏輯) =================

log "🧹 Cleaning up old logs..."
rm -fv "$SCRIPT_DIR/logs"/*.log
rm -fv "$SCRIPT_DIR/logs"/*.json
rm -fv "$CI_TARGET_DIR/test"/*.log
log "🧪 3. Pre-build Tests (testAll)..."
run_test_command "testAll" $CI_SCRIPT_NAME testAll
final_status=$?
if [ $final_status -eq 2 ] || [ $final_status -eq 3 ] ; then
    exit $final_status
fi
getlog
if scan_logs "testall"; then
    log "${GREEN}✅ Pre-build Tests Passed!${RESET}"
else
    log "${RED}⛔ Pre-build Tests Failed.${RESET}"
fi

log "🏗️ 5. Building..."
#run_quiet $CI_SCRIPT_NAME build || { log "Build 失敗"; exit 4; }

#build有發PR的NF的image
for pr_entry in "${PR_LIST[@]}"; do
    IFS=':' read -r comp id <<< "$pr_entry"
    run_quiet $CI_SCRIPT_NAME build-nf "$comp" || { log "Build $comp 失敗"; exit 4; }
done



# ================= 循環測試階段 =================
log "🚀 Starting Test Cycles..."
restore_and_build
for ENV in "${TEST_ENVS[@]}"; do
    ulcl_test_cycle "$ENV"
done

# restore_and_build

# ================= 完成階段 =================
#取得ci-test 內的logs
getlog
scan_logs
final_status=$?

log "🎉 All Tasks Completed!"
rm -f "$FAILED_LIST_FILE"
if [ $final_status -ne 0 ]; then
    exit 1
else
    exit 0
fi