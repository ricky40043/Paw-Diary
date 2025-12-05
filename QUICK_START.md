# 🚀 快速開始指南

## 立即啟動

```bash
# 1. 啟動伺服器（伺服器會在背景運行）
go run main.go &

# 2. 開啟瀏覽器
open http://localhost:8080/
```

就這麼簡單！✨

---

## 使用流程

### 方法一：透過網頁 UI（推薦）

1. 訪問 http://localhost:8080/
2. 點擊「開始使用」
3. 拖曳或選擇影片檔案（MP4/MOV/AVI）
4. 點擊「開始上傳」
5. 等待處理完成（通常幾秒鐘）
6. 查看精華片段並下載影片

### 方法二：透過 API

```bash
# 上傳影片
curl -X POST http://localhost:8080/api/v1/poc/jobs \
  -F "file=@your_video.mp4"

# 回應會返回 job_id
# {"job_id":"xxx","status":"pending"}

# 查詢處理結果
curl http://localhost:8080/api/v1/poc/jobs/{job_id}
```

---

## 主要功能

✨ **自動分析**：AI 自動分析影片中狗狗與人的互動
🎬 **智能剪輯**：自動找出精彩片段並生成精華影片
💎 **美觀介面**：現代化響應式設計
⚡ **快速處理**：通常幾秒內完成

---

## 頁面導覽

- `/` - 首頁（功能介紹）
- `/poc/jobs` - 上傳影片 & 任務列表
- `/poc/jobs/:id` - 查看任務詳情與結果

---

## 停止伺服器

```bash
# 找到並停止 Go 進程
pkill -f "go run main.go"
# 或
lsof -ti:8080 | xargs kill
```

---

## 常見問題

**Q: 伺服器啟動失敗？**
A: 檢查埠號 8080 是否被佔用，可修改 `.env` 中的 `PORT`

**Q: 影片處理失敗？**
A: 確認已安裝 FFmpeg (`brew install ffmpeg` 或 `sudo apt install ffmpeg`)

**Q: 前端顯示空白？**
A: 執行 `cd frontend && npm run build`

---

更多資訊請查看 `README.md` 和 `SETUP_COMPLETE.md`
