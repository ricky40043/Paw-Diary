#!/bin/bash

# 狗狗回憶影片自動剪輯 APP - 啟動腳本

echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║   🐕 狗狗回憶影片自動剪輯 APP - 啟動服務                      ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
echo ""

# 檢查依賴
echo "📋 檢查系統需求..."

if ! command -v go &> /dev/null; then
    echo "❌ 錯誤：未安裝 Go"
    echo "   請安裝：brew install go (macOS) 或 sudo apt install golang-go (Linux)"
    exit 1
fi

if ! command -v node &> /dev/null; then
    echo "❌ 錯誤：未安裝 Node.js"
    echo "   請安裝：brew install node (macOS) 或參考 https://nodejs.org/"
    exit 1
fi

if ! command -v ffmpeg &> /dev/null; then
    echo "❌ 錯誤：未安裝 FFmpeg"
    echo "   請安裝：brew install ffmpeg (macOS) 或 sudo apt install ffmpeg (Linux)"
    exit 1
fi

echo "✅ Go: $(go version | awk '{print $3}')"
echo "✅ Node.js: $(node --version)"
echo "✅ FFmpeg: $(ffmpeg -version | head -n 1 | awk '{print $3}')"
echo ""

# 檢查依賴是否已安裝
if [ ! -d "frontend/node_modules" ]; then
    echo "📦 安裝前端依賴..."
    cd frontend && npm install && cd ..
fi

if [ ! -f "go.sum" ]; then
    echo "📦 安裝後端依賴..."
    go mod download
fi

# 建立前端
if [ ! -d "frontend/dist" ]; then
    echo "🔨 建立前端..."
    cd frontend && npm run build && cd ..
fi

echo ""
echo "🚀 啟動服務..."
echo ""

# 清理舊的進程
pkill -f "go run main.go" 2>/dev/null
pkill -f "dog-memory-app" 2>/dev/null
pkill -f "vite" 2>/dev/null
sleep 1  # 等待進程完全終止

# 啟動後端（清除環境變數以確保讀取 .env）
echo "📡 啟動後端伺服器 (http://localhost:8080)..."
# 先編譯
go build -o dog-memory-app main.go config.go
# 用乾淨的環境啟動
env -i PATH=$PATH HOME=$HOME ./dog-memory-app > logs/backend.log 2>&1 &
BACKEND_PID=$!

# 等待後端啟動
sleep 2

# 檢查後端是否成功啟動
if ! curl -s http://localhost:8080/api/health > /dev/null; then
    echo "❌ 後端啟動失敗！查看 logs/backend.log"
    exit 1
fi

echo "✅ 後端啟動成功 (PID: $BACKEND_PID)"
echo ""

# 啟動前端開發伺服器
echo "📡 啟動前端開發伺服器 (http://localhost:3000)..."
cd frontend
npm run dev > ../logs/frontend.log 2>&1 &
FRONTEND_PID=$!
cd ..
echo "✅ 前端開發伺服器啟動成功 (PID: $FRONTEND_PID)"
echo ""

echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║   ✅ 服務啟動完成！                                            ║"
echo "╠═══════════════════════════════════════════════════════════════╣"
echo "║   🌐 前端開發介面：http://localhost:3000                      ║"
echo "║   📡 後端 API：    http://localhost:8080/api                  ║"
echo "║   📊 健康檢查：    http://localhost:8080/api/health           ║"
echo "╠═══════════════════════════════════════════════════════════════╣"
echo "║   📝 日誌位置：                                                ║"
echo "║      - 後端：logs/backend.log                                 ║"
echo "║      - 前端：logs/frontend.log                                ║"
echo "╠═══════════════════════════════════════════════════════════════╣"
echo "║   🛑 停止服務：bash stop.sh                                   ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
echo ""

echo "後端 PID: $BACKEND_PID" > logs/pids.txt
echo "前端 PID: $FRONTEND_PID" >> logs/pids.txt

# 保持腳本運行並顯示後端日誌
echo "📋 正在顯示後端日誌 (按 Ctrl+C 停止服務)..."
tail -f logs/backend.log
