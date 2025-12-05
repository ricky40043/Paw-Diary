# 📚 Phase 2 API 文檔

## Phase 2：多影片故事生成

### 核心流程

```
1. 建立專案 → 2. 上傳多個影片 → 3. 生成故事 → 4. 取得最終影片
```

---

## API 端點

### 1. 建立專案

```http
POST /api/v2/story/projects
Content-Type: application/json

{
  "name": "我的狗狗回憶",
  "dog_name": "豆豆",
  "dog_breed": "吉娃娃"
}
```

**回應：**
```json
{
  "project_id": "uuid",
  "status": "pending"
}
```

---

### 2. 上傳影片

```http
POST /api/v2/story/projects/:projectId/videos
Content-Type: multipart/form-data

videos: [file1.mp4, file2.mp4, file3.mp4...]
```

**回應：**
```json
{
  "uploaded": 3,
  "videos": [
    {
      "id": "video-uuid-1",
      "original_name": "file1.mp4",
      "duration": 30.5,
      "analyzed": false
    }
  ]
}
```

---

### 3. 生成故事

```http
POST /api/v2/story/projects/:projectId/generate
```

**回應：**
```json
{
  "project_id": "uuid",
  "status": "processing"
}
```

---

### 4. 查詢專案狀態

```http
GET /api/v2/story/projects/:projectId
```

**回應（處理中）：**
```json
{
  "id": "uuid",
  "name": "我的狗狗回憶",
  "dog_name": "豆豆",
  "dog_breed": "吉娃娃",
  "status": "analyzing",  // pending, analyzing, generating_story, generating_video, completed, failed
  "videos": [...],
  "created_at": "2024-12-05T...",
  "updated_at": "2024-12-05T..."
}
```

**回應（完成）：**
```json
{
  "id": "uuid",
  "status": "completed",
  "story": {
    "title": "豆豆的溫馨時光",
    "chapters": [
      {
        "index": 1,
        "narration": "在這個溫暖的午後，豆豆依偎在主人的懷中，享受著這份專屬的寧靜時光。",
        "video_id": "video-uuid-1",
        "start_time": 10.5,
        "end_time": 25.3,
        "duration": 14.8
      }
    ]
  },
  "final_video_url": "/storage/projects/uuid/final.mp4"
}
```

---

### 5. 列出所有專案

```http
GET /api/v2/story/projects
```

**回應：**
```json
{
  "projects": [...],
  "total": 5
}
```

---

## 處理流程詳解

### Step 1: 分析所有影片
- 每個影片獨立進行 Phase 1 的完整分析
- 抽取幀 → 分段 → AI 分析 → 找出高光片段

### Step 2: AI 生成故事
- 收集所有影片的高光片段描述
- 使用 Gemini AI 生成完整故事腳本
- 故事包含：標題 + 3-5 個章節
- 每個章節包含：旁白文字 + 對應的影片片段

### Step 3: 生成 TTS 音訊（待實作）
- 將每個章節的旁白文字轉換為語音
- 目前版本跳過此步驟

### Step 4: 合成最終影片
- 根據故事章節順序剪輯影片片段
- 使用 FFmpeg concat 拼接
- 生成最終的故事影片

---

## 狀態說明

| 狀態 | 說明 |
|------|------|
| `pending` | 專案已建立，等待上傳影片 |
| `analyzing` | 正在分析影片片段 |
| `generating_story` | 正在用 AI 生成故事 |
| `generating_video` | 正在合成最終影片 |
| `completed` | 完成 |
| `failed` | 失敗 |

---

## 使用範例

### cURL 範例

```bash
# 1. 建立專案
PROJECT_ID=$(curl -X POST http://localhost:8080/api/v2/story/projects \
  -H "Content-Type: application/json" \
  -d '{"name":"我的回憶","dog_name":"豆豆","dog_breed":"吉娃娃"}' \
  | jq -r '.project_id')

# 2. 上傳影片
curl -X POST http://localhost:8080/api/v2/story/projects/$PROJECT_ID/videos \
  -F "videos=@video1.mp4" \
  -F "videos=@video2.mp4" \
  -F "videos=@video3.mp4"

# 3. 生成故事
curl -X POST http://localhost:8080/api/v2/story/projects/$PROJECT_ID/generate

# 4. 查詢狀態（輪詢直到完成）
watch -n 2 "curl -s http://localhost:8080/api/v2/story/projects/$PROJECT_ID | jq '.status'"

# 5. 下載最終影片
curl -O http://localhost:8080/storage/projects/$PROJECT_ID/final.mp4
```

---

## 與 Phase 1 的區別

| 功能 | Phase 1 | Phase 2 |
|------|---------|---------|
| 影片數量 | 單一影片 | 多個影片 |
| 輸出 | 高光片段影片 | 完整故事影片 |
| AI 功能 | 場景分析 | 場景分析 + 故事生成 |
| 旁白 | 無 | 有（待實作 TTS）|
| 用途 | POC 測試 | 完整產品 |

---

## 限制與待實作功能

### 當前限制
- ⚠️ TTS 音訊生成尚未實作（旁白文字已生成但無語音）
- ⚠️ 影片拼接使用 `-c copy`，可能有關鍵幀問題
- ⚠️ 無資料庫持久化，重啟後資料消失

### 待實作功能
- [ ] TTS 語音生成（Google TTS / OpenAI TTS）
- [ ] 影片與音訊同步
- [ ] 字幕疊加
- [ ] 轉場效果
- [ ] 背景音樂
- [ ] 結尾圖片支援

---

**Phase 2 已完成核心功能，可以開始測試！** 🎉
