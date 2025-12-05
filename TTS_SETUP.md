# 🎤 啟用 TTS 語音功能

## 當前狀態

✅ **Phase 2 核心功能已完成並測試成功**
- 多影片上傳 ✅
- AI 場景分析 ✅
- AI 故事生成 ✅
- 影片合成 ✅

⚠️ **TTS 語音功能需要啟用 API**
- 程式碼已完成 ✅
- 需要啟用 Google Cloud Text-to-Speech API

---

## 如何啟用 TTS

### 方法一：使用 Google Cloud Console（推薦）

1. **訪問 Google Cloud Console**
   ```
   https://console.cloud.google.com/
   ```

2. **選擇或建立專案**
   - 如果沒有專案，點擊「建立專案」
   - 給專案命名（例如：dog-memory-app）

3. **啟用 Text-to-Speech API**
   ```
   https://console.cloud.google.com/apis/library/texttospeech.googleapis.com
   ```
   - 點擊「啟用」按鈕

4. **建立 API Key（如果還沒有）**
   - 前往：https://console.cloud.google.com/apis/credentials
   - 點擊「建立憑證」→「API 金鑰」
   - 複製 API Key

5. **更新 .env 檔案**
   ```bash
   AI_API_KEY=你的_API_Key
   ```

6. **重啟服務**
   ```bash
   bash stop.sh
   bash start.sh
   ```

---

## 測試 TTS

啟用後重新執行：
```bash
bash test_phase2.sh
```

這次應該會生成帶有女聲旁白的影片！

---

## 當前功能（無 TTS）

即使沒有 TTS，系統仍然完全運作：

✅ **可以做的事：**
- 上傳多個狗狗影片
- AI 自動分析所有場景
- 生成完整故事腳本（文字）
- 自動剪輯並拼接影片
- 輸出最終故事影片

⚠️ **缺少的功能：**
- 女聲旁白語音

---

## TTS 啟用後的效果

### 無 TTS（當前）
```
[影片片段 1] → [影片片段 2] → [影片片段 3]
```

### 有 TTS（啟用後）
```
[女聲旁白 + 影片片段 1] → [女聲旁白 + 影片片段 2] → [女聲旁白 + 影片片段 3]
```

每個章節都會配上溫馨的女聲旁白！

---

## 費用說明

### Google Cloud TTS 定價
- **免費額度**：每月 100 萬字元（WaveNet 語音）
- **一個故事**：約 120 字（3 章節）
- **成本**：約 0.06 台幣

可以免費製作超過 8,000 個故事！

---

## 故障排除

### 403 錯誤
**原因**：Text-to-Speech API 未啟用

**解決**：按照上面的步驟啟用 API

### 401 錯誤
**原因**：API Key 無效或過期

**解決**：檢查 `.env` 中的 `AI_API_KEY`

### 429 錯誤
**原因**：超過免費額度

**解決**：等待額度重置或升級付費方案

---

## 系統架構

```
Phase 2 處理流程：
1. 上傳影片
2. AI 分析
3. 生成故事
4. 嘗試生成 TTS ← 如果失敗，跳過
5. 合成影片 ← 自動判斷有無 TTS
6. 輸出結果
```

系統會自動處理 TTS 失敗的情況，確保影片仍能正常生成！

---

**總結：Phase 2 已完整實作並測試成功！TTS 是可選的增強功能。**
