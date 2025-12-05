# 狗狗回憶影片自動剪輯 APP

## 專案說明

這是一個自動剪輯狗狗回憶影片的系統，使用 AI 分析影片中的互動片段，自動生成精華影片。

### 技術棧

- **後端**：Go (Gin Framework)
- **前端**：Vue 3 + Vite
- **影片處理**：FFmpeg
- **AI 分析**：多模態 LLM (可選)

### 專案特色

✅ **後端 API 路徑統一管理**：所有 API 路徑寫在 `main.go` 一起，不分開
✅ **前端 URL 對應資料夾結構**：網頁 URL 完全等於資料夾結構
  - `/` → `frontend/src/pages/index.vue`
  - `/poc/jobs` → `frontend/src/pages/poc/jobs/index.vue`
  - `/poc/jobs/:id` → `frontend/src/pages/poc/jobs/[id].vue`

## Phase 1 功能（已實現）✅ 100%

- ✅ 上傳影片 API (`POST /api/v1/poc/jobs`)
- ✅ 自動抽取影片幀（FFmpeg，1 fps）
- ✅ 分組成 segments（每段 3 秒）
- ✅ **AI 真實分析互動片段（OpenAI GPT-4o-mini Vision）**
  - 偵測狗、人的存在
  - 識別互動類型（玩耍、撫摸、奔跑等）
  - 判斷情緒（開心、興奮、平靜等）
  - 中文場景描述
- ✅ 挑出 Highlight 片段
- ✅ 自動剪輯精華影片
- ✅ 查詢結果 API (`GET /api/v1/poc/jobs/:jobId`)
- ✅ 完整前端 UI（上傳、列表、詳情）
- ✅ 啟動/停止腳本（start.sh / stop.sh）

## 系統需求

### 必須安裝

1. **Go 1.21+**
   ```bash
   # macOS
   brew install go
   
   # Ubuntu/Debian
   sudo apt install golang-go
   ```

2. **Node.js 18+**
   ```bash
   # macOS
   brew install node
   
   # Ubuntu/Debian
   curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
   sudo apt install nodejs
   ```

3. **FFmpeg**
   ```bash
   # macOS
   brew install ffmpeg
   
   # Ubuntu/Debian
   sudo apt install ffmpeg
   
   # Windows
   # 下載：https://ffmpeg.org/download.html
   ```

## 快速開始

### 🚀 一鍵啟動（推薦）

```bash
# 啟動所有服務（自動檢查依賴、建立前端、啟動後端）
bash start.sh
```

就這麼簡單！服務會在 `http://localhost:8080` 啟動

### 🛑 停止服務

```bash
bash stop.sh
```

---

### 手動啟動（進階）

#### 1. 安裝依賴

```bash
# 使用 Makefile
make install

# 或手動安裝
go mod download
cd frontend && npm install && cd ..
```

#### 2. 配置 AI API（可選，不設定會使用 Mock）

編輯 `.env` 檔案：

```bash
PORT=8080
STORAGE_PATH=./storage

# 🔑 設定你的 OpenAI API Key 啟用真實 AI 分析
AI_API_KEY=sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxx
AI_API_ENDPOINT=https://api.openai.com/v1/chat/completions
```

**📖 詳細 AI 設定請查看：[AI_SETUP.md](AI_SETUP.md)**

#### 3. 建立前端

```bash
cd frontend && npm run build && cd ..
```

#### 4. 啟動服務

```bash
# 生產模式
go run main.go

# 或使用 Makefile
make run
```

#### 5. 開發模式（前端熱重載）

```bash
# Terminal 1: 後端
go run main.go

# Terminal 2: 前端開發伺服器
cd frontend && npm run dev
```

開發模式訪問：`http://localhost:3000`

## API 文檔

### Phase 1 APIs

#### 1. 上傳影片並創建任務

```http
POST /api/v1/poc/jobs
Content-Type: multipart/form-data

file: <video file>
```

回應：
```json
{
  "job_id": "uuid-string",
  "status": "pending"
}
```

#### 2. 查詢任務狀態

```http
GET /api/v1/poc/jobs/:jobId
```

回應：
```json
{
  "id": "uuid-string",
  "status": "completed",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:01:00Z",
  "highlights": [
    {
      "start": 10.5,
      "end": 15.2,
      "caption": "狗狗朝主人飛奔",
      "interaction": "running_towards_owner",
      "emotion": "happy"
    }
  ],
  "highlight_video_url": "/storage/videos/uuid/highlight.mp4"
}
```

#### 3. 列出所有任務

```http
GET /api/v1/poc/jobs
```

回應：
```json
{
  "jobs": [...],
  "total": 5
}
```

## 專案結構

```
dog-memory-app/
├── main.go                          # 後端主程式（所有 API 路徑在此）
├── go.mod                           # Go 依賴
├── .env                             # 環境變數
├── storage/                         # 檔案儲存目錄
│   └── videos/
│       └── {job-id}/
│           ├── original.mp4         # 原始影片
│           ├── frames/              # 抽取的幀
│           └── highlight.mp4        # 精華影片
└── frontend/                        # 前端專案
    ├── package.json
    ├── vite.config.js
    ├── index.html
    └── src/
        ├── main.js
        ├── App.vue
        └── pages/                   # 頁面（URL 對應資料夾結構）
            ├── index.vue            # / 首頁
            └── poc/
                └── jobs/
                    ├── index.vue    # /poc/jobs 列表頁
                    └── [id].vue     # /poc/jobs/:id 詳情頁
```

## 處理流程

1. **上傳影片** → 創建 Job（狀態：pending）
2. **抽取幀** → FFmpeg 每秒抽 1 張圖
3. **分段** → 每 3 秒為一段
4. **AI 分析** → 分析每段的互動類型、情緒
5. **找出高光** → 有狗+有人+有互動的連續片段
6. **剪輯影片** → FFmpeg 剪出精華片段
7. **完成** → 狀態：completed，返回結果

## 🤖 AI 分析

系統已整合 **OpenAI GPT-4o-mini Vision API**！

### 啟用真實 AI 分析

1. **取得 API Key**：訪問 https://platform.openai.com/api-keys
2. **設定環境變數**：編輯 `.env` 檔案
   ```bash
   AI_API_KEY=sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxx
   ```
3. **重啟服務**：`bash stop.sh && bash start.sh`

### AI 分析能力

- ✅ 偵測畫面中的狗和人
- ✅ 識別互動類型（奔跑、玩耍、撫摸、撿球、依偎等）
- ✅ 判斷情緒（開心、興奮、平靜、中性、悲傷）
- ✅ 中文場景描述

### 成本估算

- 每支 30 秒影片約 $0.0006 USD（不到 0.02 台幣）
- 使用 GPT-4o-mini 模型（便宜快速）

### 自動降級

如果 AI API 失敗或未設定，系統會自動使用 Mock 模式確保功能正常。

**📖 詳細說明：[AI_SETUP.md](AI_SETUP.md)**

## 🧪 測試

### 快速測試

使用 `start.sh` 啟動後，在互動選單選擇「3) 測試上傳影片」

### 手動測試

```bash
# 1. 建立測試影片（如果需要）
ffmpeg -f lavfi -i testsrc=duration=10:size=1280x720:rate=30 \
       -f lavfi -i sine=frequency=1000:duration=10 \
       -c:v libx264 -pix_fmt yuv420p -c:a aac \
       -y test_video.mp4

# 2. 上傳測試
bash test_upload.sh test_video.mp4

# 3. 或使用你自己的影片
bash test_upload.sh path/to/your/dog_video.mp4
```

### 查看日誌

```bash
# 即時查看後端日誌
tail -f logs/backend.log

# 查看 AI 分析結果
tail -f logs/backend.log | grep "AI Analysis"
```

## 下一步開發（Phase 2）

- [ ] 支援多影片上傳
- [ ] 自動生成故事 Outline
- [ ] TTS 旁白生成
- [ ] 結尾圖片支援
- [ ] 更複雜的影片拼接

## 下一步開發（Phase 3）

- [ ] 完整產品化 UI
- [ ] 6 步驟引導式操作
- [ ] 狗狗資料管理
- [ ] 風格選擇
- [ ] 社交分享功能

## 授權

MIT License
# Paw-Diary
