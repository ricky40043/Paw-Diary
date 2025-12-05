# 🎤 TTS 語音功能說明

## ✅ 已完成實作

Phase 2 現在已完整支援 **TTS 語音旁白功能**！

---

## 🎯 功能概述

系統會自動：
1. **生成故事腳本**（AI 生成旁白文字）
2. **轉換為語音**（Google Cloud Text-to-Speech）
3. **同步音訊與影片**（自動調整影片速度以匹配旁白時長）
4. **合成最終影片**（旁白 + 影片片段）

---

## 🔧 技術細節

### 使用的 TTS 服務
- **Google Cloud Text-to-Speech API**
- 使用與 Gemini 相同的 API Key
- 語音模型：`cmn-TW-Wavenet-A`（台灣中文女聲）

### 語音設定
```json
{
  "voice": {
    "languageCode": "zh-TW",
    "name": "cmn-TW-Wavenet-A",
    "ssmlGender": "FEMALE"
  },
  "audioConfig": {
    "audioEncoding": "MP3",
    "speakingRate": 0.95,  // 稍微慢一點，更溫暖
    "pitch": 0.0
  }
}
```

### 音訊與影片同步

系統會智能調整影片播放速度以匹配旁白時長：

```
如果 旁白時長 > 影片時長
  → 減慢影片播放（讓影片拉長以配合旁白）

如果 旁白時長 < 影片時長  
  → 加快影片播放（讓影片縮短以配合旁白）

速度限制：0.5x - 2.0x（避免過度失真）
```

**範例：**
- 影片片段：10 秒
- 旁白時長：8 秒
- 調整速度：1.25x（加快播放）

---

## 📊 處理流程

### 完整流程（含 TTS）

```
1. 建立專案
   ↓
2. 上傳多個影片
   ↓
3. 分析所有影片（AI 視覺分析）
   ↓
4. 生成故事腳本（Gemini AI）
   ├─ 標題
   ├─ 章節 1：旁白文字 + 對應影片片段
   ├─ 章節 2：旁白文字 + 對應影片片段
   └─ 章節 3：旁白文字 + 對應影片片段
   ↓
5. 生成 TTS 音訊（每個章節）
   ├─ 章節 1 → chapter_1.mp3 (8.5s)
   ├─ 章節 2 → chapter_2.mp3 (7.2s)
   └─ 章節 3 → chapter_3.mp3 (9.1s)
   ↓
6. 合成影片
   ├─ 剪出影片片段 1
   ├─ 調整速度以匹配音訊 1
   ├─ 合併音訊 1 + 影片 1
   ├─ （重複其他章節）
   └─ 拼接所有章節
   ↓
7. 輸出最終影片（帶旁白）
```

---

## 🎬 實際效果

### 輸入
```json
{
  "story": {
    "title": "豆豆的溫馨時光",
    "chapters": [
      {
        "narration": "在這個溫暖的午後，豆豆依偎在主人的懷中，享受著這份專屬的寧靜時光。",
        "video": "video1.mp4 (10s-25s)"
      },
      {
        "narration": "陽光灑落，豆豆興奮地與主人玩耍，尾巴搖得像一朵綻放的花。",
        "video": "video2.mp4 (5s-18s)"
      }
    ]
  }
}
```

### 輸出
**最終影片結構：**
```
[章節 1]
- 旁白：「在這個溫暖的午後，豆豆依偎在主人的懷中...」（女聲旁白）
- 影片：豆豆被撫摸的片段（自動調整速度以匹配旁白時長）

[章節 2]  
- 旁白：「陽光灑落，豆豆興奮地與主人玩耍...」（女聲旁白）
- 影片：豆豆玩耍的片段（自動調整速度）

→ 自動拼接成一個完整的故事影片
```

---

## 💰 費用說明

### Google Cloud TTS 定價

**免費額度：**
- 每月前 100 萬字元免費（WaveNet 語音）
- 每月前 400 萬字元免費（標準語音）

**付費方案：**
- WaveNet 語音：$0.000016 USD / 字元
- 標準語音：$0.000004 USD / 字元

### 成本估算

**一個專案（3 個章節）：**
- 每個章節約 30-50 字
- 總計約 120 字
- 成本：$0.00192 USD（約 0.06 台幣）

**非常便宜！** ✨

---

## 🔊 音訊檔案管理

### 儲存位置
```
storage/
└── projects/
    └── {project-id}/
        ├── audio/
        │   ├── chapter_1.mp3
        │   ├── chapter_2.mp3
        │   └── chapter_3.mp3
        ├── segment_1.mp4
        ├── segment_2.mp4
        ├── segment_3.mp4
        └── final.mp4
```

### 音訊格式
- **編碼**：MP3
- **取樣率**：24kHz（Google TTS 預設）
- **位元率**：自適應

---

## 🎨 可自訂選項

### 語音風格

可以修改 `main.go` 中的 TTS 設定：

```go
"voice": map[string]interface{}{
    "languageCode": "zh-TW",
    "name":         "cmn-TW-Wavenet-A",  // 更改這裡
    "ssmlGender":   "FEMALE",
}
```

**可選語音：**
- `cmn-TW-Wavenet-A` - 女聲（溫柔）
- `cmn-TW-Wavenet-B` - 男聲（沉穩）
- `cmn-TW-Wavenet-C` - 男聲（年輕）

### 語速和音調

```go
"audioConfig": map[string]interface{}{
    "speakingRate": 0.95,  // 0.25 - 4.0（0.95 = 稍慢）
    "pitch":        0.0,   // -20.0 到 20.0（0 = 正常）
}
```

---

## 🐛 錯誤處理

### TTS 失敗處理

如果 TTS 生成失敗（例如：API 錯誤、額度不足），系統會：
1. 記錄警告訊息
2. 繼續處理其他章節
3. 生成沒有旁白的影片（只有影片片段）

**日誌範例：**
```
Warning: TTS generation failed for chapter 1: TTS API error 429
```

### 音訊與影片不同步

如果音訊時長與影片時長差異太大：
- 系統會自動調整影片速度（0.5x - 2.0x）
- 超過範圍會裁切或填充黑畫面

---

## 📈 效能數據

### 處理時間（估算）

**3 個影片，總長 2 分鐘：**
- 影片分析：約 30 秒
- 故事生成：約 10 秒
- TTS 生成（3 個章節）：約 15 秒
- 影片合成：約 20 秒
- **總計：約 75 秒**

---

## 🎯 使用範例

### API 調用（完整流程）

```bash
# 1. 建立專案
PROJECT_ID=$(curl -X POST http://localhost:8080/api/v2/story/projects \
  -H "Content-Type: application/json" \
  -d '{"name":"回憶","dog_name":"豆豆","dog_breed":"吉娃娃"}' \
  | jq -r '.project_id')

# 2. 上傳影片
curl -X POST http://localhost:8080/api/v2/story/projects/$PROJECT_ID/videos \
  -F "videos=@video1.mp4" \
  -F "videos=@video2.mp4"

# 3. 生成故事（自動包含 TTS）
curl -X POST http://localhost:8080/api/v2/story/projects/$PROJECT_ID/generate

# 4. 查詢結果
curl http://localhost:8080/api/v2/story/projects/$PROJECT_ID | jq .

# 5. 下載最終影片（帶旁白）
curl -O http://localhost:8080/storage/projects/$PROJECT_ID/final.mp4
```

---

## ✨ 特色功能

### 1. 智能速度調整
自動調整影片播放速度以完美配合旁白時長

### 2. 溫暖的女聲旁白
使用 WaveNet 高品質語音，聽起來自然溫暖

### 3. 無縫拼接
各章節之間平滑轉場

### 4. 錯誤容錯
即使 TTS 失敗也能生成影片

---

## 🔧 故障排除

### 問題：TTS API 錯誤 403

**原因**：API Key 沒有啟用 Text-to-Speech API

**解決方法**：
1. 訪問 [Google Cloud Console](https://console.cloud.google.com/)
2. 啟用 Cloud Text-to-Speech API
3. 確認 API Key 有權限

### 問題：音訊與影片不同步

**原因**：影片時長與旁白時長差異太大

**解決方法**：
- 調整 AI 生成的故事，讓旁白更符合影片時長
- 或手動調整速度限制範圍

---

**TTS 功能已完整實作並測試！準備好創作你的狗狗故事影片了嗎？** 🎬🐕💕
