# 狗狗回憶影片自動剪輯 APP — 開發製作步驟文件

> 目標：從零開始，分三個階段完成「狗狗回憶影片自動剪輯」系統  
> 分為：概念驗證（Phase 1）→ 多影片故事生成（Phase 2）→ 產品化 APP（Phase 3）

---

## 0. 基礎技術選擇（建議）

可依自己習慣調整，這裡用一套「典型但好維護」的組合：

- Backend 語言：Go / Python / .NET 任一  
- 任務隊列：Redis + background worker 或資料表 polling  
- 檔案儲存：本機磁碟 →（之後可換 S3 / GCS）  
- 影片處理：FFmpeg  
- AI 模型：多模態 LLM（看圖）、TTS（文字轉語音）

---

# Phase 1：概念驗證（單一影片 → 自動找到互動片段）

## Phase 1-1：專案初始化

- 建立 Backend 專案  
- 建資料夾結構  
- 設環境變數（AI_API_KEY / STORAGE 路徑等）

---

## Phase 1-2：上傳影片 API（單支）

### POST `/api/v1/poc/jobs`

- multipart/form-data  
- 上傳 file.mp4  
- 回傳 jobId（pending）

---

## Phase 1-3：抽 Frame

- 使用 FFmpeg：

```
ffmpeg -i input.mp4 -vf fps=1 frames/output_%04d.jpg
```

- 建立 frame list（時間 + path）

---

## Phase 1-4：分組成 segment

- 每段 2–4 秒  
- 每段包含 N 張圖片  

範例：

```
[
  { segmentIndex: 1, start:0.0, end:2.0, framePaths:[...] },
  { segmentIndex: 2, start:2.0, end:4.0, framePaths:[...] }
]
```

---

## Phase 1-5：丟給多模態 LLM 做動作分析

Prompt 要求模型回：

```
{
  "has_dog": true,
  "has_human": true,
  "interaction_type": "running_towards_owner",
  "emotion": "happy",
  "short_caption": "狗狗朝主人飛奔"
}
```

---

## Phase 1-6：挑出 Highlight 片段

- 規則：  
  - hasDog = true  
  - hasHuman = true  
  - interaction_type ≠ none  
- 找連續 segment → 合併成一段

使用 FFmpeg 剪：

```
ffmpeg -i input.mp4 -ss {start} -to {end} -c copy highlight.mp4
```

---

## Phase 1-7：查詢結果 API

### GET `/api/v1/poc/jobs/{jobId}`

回傳 highlight 起訖秒數 + 描述 + highlightVideoUrl

---

# Phase 2：多影片 → 自動生成完整故事

## Phase 2-1：Project 概念

新增 API：

- `POST /api/v2/story/projects`（建立專案）  
- `POST /api/v2/story/projects/{id}/videos`（上傳多影片）  
- `POST /api/v2/story/projects/{id}/ending-image`  
- `POST /api/v2/story/projects/{id}/generate`

---

## Phase 2-2：多影片批次分析

對每支影片：

- 抽 frame  
- 分段  
- 丟給 LLM 分析  
- 收集所有高互動 segments

儲存：

```
[
 { videoId:"vid1", start:12.0, end:16.0, interaction:"running", emotion:"happy", caption:"狗狗衝向主人" },
 ...
]
```

---

## Phase 2-3：LLM 生成故事 Outline

LLM 输入內容：

- 狗狗資訊（名字、年齡、是否離世、個性）  
- 使用者選擇風格（warm / healing / funny）  
- 所有 segments 的 caption + 情緒資料  
- 使用者影片總長度偏好（例如 60 秒）

LLM 回傳：

```
{
 "outline": [
   {
     "order": 1,
     "type": "video_segment",
     "fromVideoId": "vid1",
     "startTime": 10.0,
     "endTime": 16.0,
     "voiceText": "每天下班你都衝來迎接我。",
     "subtitleText": "每天下班你都衝來迎接我。"
   },
   {
     "order": 99,
     "type": "ending_image",
     "imageSource": "project_image",
     "voiceText": "謝謝你陪我走過人生。",
     "subtitleText": "謝謝你陪我走過人生。"
   }
 ]
}
```

---

## Phase 2-4：依 Outline 用 FFmpeg 拼成影片

1. 逐段剪出 part_001.mp4、part_002.mp4  
2. concat 成影片：

```
ffmpeg -f concat -safe 0 -i list.txt -c copy story_raw.mp4
```

3. 若使用 TTS → 合成旁白音訊 → mux 進影片：

```
ffmpeg -i story_raw.mp4 -i voice.mp3 -map 0:v -map 1:a -c:v copy -c:a aac story_final.mp4
```

---

## Phase 2-5：結果 API

### GET `/api/v2/story/projects/{id}`

回傳：

- finalVideoUrl  
- outline JSON  
- duration  
- dogProfile  
- style  

---

# Phase 3：產品化（引導式 UI + APP / Web）

## Phase 3-1：使用者完整流程

1. 填寫狗狗資料  
2. 上傳 1–5 支影片  
3. 上傳結尾照片  
4. 選擇影片風格與選項（是否配音、影片長度）  
5. 送出 → 生成中動畫（顯示暫存文字）  
6. 生成完成 → 播放、下載、分享

---

## Phase 3-2：前端要做的事情

- Step Wizard UI（6 步驟）  
- 呼叫 Phase 2 的 API  
- 輪詢生成中狀態  
- 前端根據 outline 顯示資訊（例如影片每段的小標題）  

---

# 最終完成標記（MVP 完成）

✓ 單支影片自動分析互動片段  
✓ 多支影片自動生成完整故事  
✓ 支援結尾圖片＋旁白  
✓ FFmpeg 自動剪輯  
✓ 前端引導式操作  
✓ 回傳完整影片（雲端 URL）

---

如需 PDF / DOCX 版本，也能直接輸出。
