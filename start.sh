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
pkill -f "vite" 2>/dev/null

# 啟動後端（清除環境變數以確保讀取 .env）
echo "📡 啟動後端伺服器 (http://localhost:8080)..."
# 先編譯
go build -o dog-memory-app main.go
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

# 詢問是否啟動前端開發伺服器
read -p "🎨 是否啟動前端開發伺服器 (熱重載)? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
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
else
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════╗"
    echo "║   ✅ 後端服務啟動完成！                                        ║"
    echo "╠═══════════════════════════════════════════════════════════════╣"
    echo "║   🌐 前端介面：    http://localhost:8080                      ║"
    echo "║   📡 後端 API：    http://localhost:8080/api                  ║"
    echo "║   📊 健康檢查：    http://localhost:8080/api/health           ║"
    echo "╠═══════════════════════════════════════════════════════════════╣"
    echo "║   📝 日誌位置：logs/backend.log                               ║"
    echo "║   🛑 停止服務：bash stop.sh                                   ║"
    echo "╚═══════════════════════════════════════════════════════════════╝"
    echo ""
    echo "後端 PID: $BACKEND_PID" > logs/pids.txt
fi

# 保持腳本運行
echo "💡 提示：按 Ctrl+C 可查看日誌或停止服務"
echo ""

# 選擇操作
while true; do
    echo "選擇操作："
    echo "  1) 查看後端日誌"
    echo "  2) 查看前端日誌"
    echo "  3) 測試上傳影片"
    echo "  4) 打開瀏覽器"
    echo "  5) 停止所有服務並退出"
    echo ""
    read -p "請選擇 [1-5]: " choice
    
    case $choice in
        1)
            echo "=== 後端日誌 (按 Ctrl+C 返回) ==="
            tail -f logs/backend.log
            ;;
        2)
            if [ -f "logs/frontend.log" ]; then
                echo "=== 前端日誌 (按 Ctrl+C 返回) ==="
                tail -f logs/frontend.log
            else
                echo "前端開發伺服器未啟動"
            fi
            ;;
        3)
            if [ -f "tmp_rovodev_test_video.mp4" ]; then
                bash test_upload.sh tmp_rovodev_test_video.mp4
            else
                echo "建立測試影片..."
                ffmpeg -f lavfi -i testsrc=duration=10:size=1280x720:rate=30 \
                       -f lavfi -i sine=frequency=1000:duration=10 \
                       -c:v libx264 -pix_fmt yuv420p -c:a aac \
                       -y tmp_rovodev_test_video.mp4 2>/dev/null
                bash test_upload.sh tmp_rovodev_test_video.mp4
            fi
            ;;
        4)
            if [[ "$OSTYPE" == "darwin"* ]]; then
                open http://localhost:8080/
            elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
                xdg-open http://localhost:8080/ 2>/dev/null || echo "請手動開啟: http://localhost:8080/"
            else
                echo "請手動開啟: http://localhost:8080/"
            fi
            ;;
        5)
            echo "🛑 停止所有服務..."
            bash stop.sh
            exit 0
            ;;
        *)
            echo "無效選擇"
            ;;
    esac
    echo ""
done
