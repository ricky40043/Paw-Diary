# 🤖 AI API 設定指南

## 概述

本系統已整合 **OpenAI GPT-4 Vision API**，可以真實分析影片中的狗狗互動片段。

---

## 🔑 取得 OpenAI API Key

### 步驟 1：註冊 OpenAI 帳號

1. 訪問 https://platform.openai.com/
2. 註冊或登入帳號

### 步驟 2：取得 API Key

1. 進入 https://platform.openai.com/api-keys
2. 點擊「Create new secret key」
3. 複製生成的 API Key（格式：`sk-...`）
4. **重要**：妥善保管，離開頁面後無法再查看

### 步驟 3：設定付費方式

1. 進入 https://platform.openai.com/account/billing
2. 設定信用卡
3. 建議設定使用限額（Usage limits）避免超支

---

## ⚙️ 設定 API Key

### 方法 1：修改 .env 檔案（推薦）

```bash
# 編輯 .env 檔案
nano .env

# 或使用任何文字編輯器
code .env
```

修改以下內容：

```bash
PORT=8080
STORAGE_PATH=./storage

# 將這裡改成你的真實 API Key
AI_API_KEY=sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# API 端點（通常不需要改）
AI_API_ENDPOINT=https://api.openai.com/v1/chat/completions
```

### 方法 2：環境變數

```bash
export AI_API_KEY="sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export AI_API_ENDPOINT="https://api.openai.com/v1/chat/completions"
```

---

## 🧪 測試 AI 功能

### 1. 重新啟動服務

```bash
# 停止舊服務
bash stop.sh

# 啟動新服務
bash start.sh
```

### 2. 上傳測試影片

```bash
# 建立測試影片（如果還沒有）
ffmpeg -f lavfi -i testsrc=duration=10:size=1280x720:rate=30 \
       -f lavfi -i sine=frequency=1000:duration=10 \
       -c:v libx264 -pix_fmt yuv420p -c:a aac \
       -y test_video.mp4

# 上傳測試
bash test_upload.sh test_video.mp4
```

### 3. 查看日誌

```bash
# 查看後端日誌，應該看到 AI 分析訊息
tail -f logs/backend.log
```

應該看到類似訊息：
```
Using real AI analysis for job xxx
AI Analysis: has_dog=true, has_human=false, interaction=none, emotion=neutral, caption=彩色測試畫面
```

---

## 💰 費用說明

### GPT-4o-mini 定價（2024）

- **輸入**：$0.150 / 1M tokens
- **輸出**：$0.600 / 1M tokens
- **圖片**：約 85 tokens per image (低細節)

### 估算範例

**處理 1 支 30 秒影片：**
- 抽取 30 幀（1 fps）
- 分成 10 個 segments
- 分析 10 張圖片

**費用計算：**
- 每張圖片 prompt: ~200 tokens
- 每張圖片: ~85 tokens
- 每張回應: ~50 tokens
- 總計: (200 + 85 + 50) × 10 = 3,350 tokens

**成本：約 $0.0006 USD（不到 0.02 台幣）**

### 成本優化建議

1. **降低抽幀率**：改為 0.5 fps（每 2 秒 1 幀）
2. **只分析關鍵幀**：每個 segment 只取中間 1 幀（已實作）
3. **設定 API 限額**：在 OpenAI 後台設定每月上限
4. **批次處理**：未來可以批次發送多張圖片

---

## 🔍 AI 分析能力

### 目前可以識別

✅ **狗的存在**
- 偵測畫面中是否有狗
- 多種品種都可識別

✅ **人的存在**
- 偵測畫面中是否有人

✅ **互動類型**
- `running_towards_owner` - 朝主人跑
- `playing` - 玩耍
- `being_petted` - 被撫摸
- `fetching` - 撿球/玩具
- `cuddling` - 依偎
- `none` - 無明顯互動

✅ **情緒判斷**
- `happy` - 開心
- `excited` - 興奮
- `calm` - 平靜
- `neutral` - 中性
- `sad` - 悲傷

✅ **場景描述**
- 中文簡短描述（10 字以內）

---

## ⚡ 效能與速度

### 處理時間

- **Mock 模式**：< 1 秒（10 秒影片）
- **AI 模式**：約 5-10 秒（10 秒影片）
  - 每個 segment 延遲 500ms 避免限流
  - API 請求時間約 1-2 秒/張圖片

### 優化方式

1. **並行處理**（未來）
   ```go
   // 使用 goroutine 同時分析多個 segments
   ```

2. **快取機制**（未來）
   ```go
   // 相似圖片不重複分析
   ```

---

## 🛡️ 錯誤處理

### 自動降級

如果 AI API 失敗，系統會自動退回到 Mock 模式：

```go
if err != nil {
    log.Printf("AI analysis failed, using mock: %v", err)
    return mockAnalyzeSegments(job)
}
```

### 常見錯誤

#### 1. API Key 無效
```
API error 401: Incorrect API key provided
```
**解決**：檢查 `.env` 中的 `AI_API_KEY` 是否正確

#### 2. 額度不足
```
API error 429: You exceeded your current quota
```
**解決**：
- 檢查 OpenAI 帳單設定
- 充值或設定付費方式

#### 3. 速率限制
```
API error 429: Rate limit exceeded
```
**解決**：
- 系統已有 500ms 延遲
- 可以增加延遲時間
- 升級 OpenAI 方案

#### 4. 網路逾時
```
failed to send request: timeout
```
**解決**：
- 檢查網路連線
- 增加 timeout 時間（目前 30 秒）

---

## 🔧 進階設定

### 更換 AI 模型

在 `main.go` 中修改：

```go
"model": "gpt-4o-mini",  // 更改這裡
```

可用模型：
- `gpt-4o-mini` - 便宜快速（推薦）
- `gpt-4o` - 更準確但較貴
- `gpt-4-turbo` - 平衡選擇

### 自訂 Prompt

修改 `analyzeSegmentWithAI` 函數中的 prompt 文字，可以：
- 調整判斷標準
- 增加更多互動類型
- 調整描述風格

### 調整分析頻率

```go
// 改為每 2 秒分析一次
segments := []Segment{}
segmentSize := 2  // 原本是 3
```

---

## 📊 監控與日誌

### 查看 AI 分析日誌

```bash
# 即時查看
tail -f logs/backend.log | grep "AI Analysis"

# 搜尋特定任務
grep "job-id-here" logs/backend.log
```

### 日誌內容範例

```
2024/12/03 20:53:29 Using real AI analysis for job xxx
2024/12/03 20:53:31 AI Analysis: has_dog=true, has_human=true, interaction=playing, emotion=happy, caption=狗狗玩球
2024/12/03 20:53:32 AI Analysis: has_dog=true, has_human=false, interaction=none, emotion=calm, caption=狗狗休息
2024/12/03 20:53:34 AI analyzed 4 segments for job xxx
```

---

## 🔒 安全注意事項

### ⚠️ 重要提醒

1. **不要提交 API Key 到 Git**
   - `.env` 已加入 `.gitignore`
   - 確認不要 commit 真實 API Key

2. **設定使用限額**
   - 在 OpenAI 後台設定每月上限
   - 建議先設 $5-10 測試

3. **監控用量**
   - 定期檢查 https://platform.openai.com/usage
   - 注意異常用量

4. **API Key 輪換**
   - 定期更換 API Key
   - 如果外洩立即作廢並重新生成

---

## 🆚 Mock vs AI 模式比較

| 功能 | Mock 模式 | AI 模式 |
|------|-----------|---------|
| 速度 | ⚡ 極快 (<1秒) | 🐢 較慢 (5-10秒) |
| 費用 | 💰 免費 | 💳 需付費 ($0.0006/影片) |
| 準確度 | ⚠️ 假數據 | ✅ 真實分析 |
| 網路需求 | ❌ 不需要 | ✅ 需要 |
| 適用場景 | 開發測試 | 正式使用 |

---

## 📞 故障排除

### 問題：一直使用 Mock 模式

**檢查清單：**
1. `.env` 中 `AI_API_KEY` 是否設定？
2. API Key 格式是否正確（`sk-...`）？
3. 服務是否重新啟動？

```bash
# 檢查環境變數
cat .env | grep AI_API_KEY

# 重新啟動
bash stop.sh && bash start.sh
```

### 問題：AI 分析失敗

**檢查清單：**
1. 網路是否正常？
2. OpenAI 服務是否正常？
3. API Key 是否有額度？

```bash
# 測試 API Key
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $AI_API_KEY"
```

---

## 🎯 下一步

設定完成後：
1. ✅ 上傳真實的狗狗影片測試
2. ✅ 查看 AI 分析結果是否準確
3. ✅ 根據結果調整 prompt
4. ✅ 開始開發 Phase 2 功能

---

**需要幫助？**
- OpenAI 文檔：https://platform.openai.com/docs
- Vision API 指南：https://platform.openai.com/docs/guides/vision
