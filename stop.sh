#!/bin/bash

# 停止所有服務

echo "🛑 停止狗狗回憶影片自動剪輯 APP 服務..."

# 從 PID 檔案讀取並停止
if [ -f "logs/pids.txt" ]; then
    while IFS= read -r line; do
        PID=$(echo $line | awk '{print $NF}')
        if ps -p $PID > /dev/null 2>&1; then
            echo "停止進程 $PID..."
            kill $PID 2>/dev/null
        fi
    done < logs/pids.txt
    rm logs/pids.txt
fi

# 強制停止相關進程
pkill -f "go run main.go" 2>/dev/null
pkill -f "dog-memory-app" 2>/dev/null
pkill -f "vite" 2>/dev/null

# 釋放端口
lsof -ti:8080 | xargs kill -9 2>/dev/null
lsof -ti:3000 | xargs kill -9 2>/dev/null

echo "✅ 所有服務已停止"
