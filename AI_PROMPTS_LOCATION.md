# AI Prompts 位置說明

本文檔列出所有使用 AI 生成內容的 prompt 位置，方便你自行修改提示詞。

---

## 1. 影片分析 Prompt (Gemini Vision API)
**文件**: `main.go`  
**函數**: `analyzeVideoWithAI`  
**行數**: 約 776-825 行

```go
prompt := fmt.Sprintf(`請仔細觀察這段影片，描述看到的內容。

影片資訊：
- 狗狗名字：%s
- 品種：%s
- 飼主關係：%s

請描述：
1. 狗狗在做什麼？
2. 環境和場景是什麼樣子？
3. 有什麼特別的細節或有趣的地方？
4. 整體氛圍是什麼感覺？

請用 2-3 句話描述，語氣溫暖自然。`,
		project.DogName,
		project.DogBreed,
		project.OwnerRelationship,
	)
```

**用途**: 分析上傳的影片內容，生成影片描述

---

## 2. 故事生成 Prompt (Gemini API)
**文件**: `main.go`  
**函數**: `generateStoryWithAI`  
**行數**: 約 1060-1145 行

```go
prompt := fmt.Sprintf(`你是一位溫暖的故事講述者，請根據以下狗狗的影片片段，創作一個溫馨感人的故事。

狗狗資訊：
- 名字：%s
- 品種：%s
- 與飼主的關係：%s

影片片段描述：
%s

請創作一個約 100-150 字的故事，要求：
1. 以第三人稱敘述
2. 溫暖、感性、充滿愛
3. 可以加入一些想像，但要基於影片內容
4. 自然銜接各個場景
5. 結尾要溫馨感人

故事格式：
直接輸出故事文字，不要有任何標題或說明。`,
		project.DogName,
		project.DogBreed,
		ownerTitle,
		strings.Join(videoDescriptions, "\n"),
	)
```

**用途**: 根據影片描述生成完整的故事旁白

---

## 3. 故事分段 Prompt (Gemini API)
**文件**: `main.go`  
**函數**: `generateStoryWithAI` (同上)  
**行數**: 約 1200-1280 行

```go
segmentPrompt := fmt.Sprintf(`請將以下故事分成 %d 個段落，每個段落對應一個影片片段。

故事：
%s

影片片段資訊：
%s

要求：
1. 總共分成 %d 段
2. 每段長度儘量平均
3. 確保段落在合理的地方斷開（完整的句子或意思）
4. 每段要能獨立理解，但整體連貫

請用以下 JSON 格式回答：
{
  "chapters": [
    {
      "index": 1,
      "text": "第一段文字...",
      "video_index": 0
    },
    {
      "index": 2,
      "text": "第二段文字...",
      "video_index": 1
    }
  ]
}

只回傳 JSON，不要有其他文字。`,
		numVideos,
		story,
		videoInfo,
		numVideos,
	)
```

**用途**: 將完整故事分段，對應到各個影片片段

---

## 4. 狗狗回應生成 Prompt (Gemini API)
**文件**: `main.go`  
**函數**: `generateDogResponse`  
**行數**: 約 1357-1430 行

```go
prompt := fmt.Sprintf(`你是一隻名叫「%s」的%s狗狗。你的%s剛剛對你說了一段很溫暖的話，現在輪到你用狗狗的視角回應了。

%s對你說：
「%s」

你們之間的美好回憶：
%s

請以狗狗的第一人稱（我）回應，要求：
1. **根據%s說的具體內容智能回應**，不要只說「我愛你」這種簡短的話
2. 字數：30-50 字
3. 語氣：溫暖、感性、真摯，像是狗狗真的在對%s說心裡話
4. 可以提到你們之間的回憶、日常相處、或者表達感激之情
5. 不要用「汪汪」「嗚嗚」等擬聲詞，用正常的中文表達
6. 要有感情深度，讓人感動

範例回應：
- 如果%s說「謝謝你陪伴我」→ 回應：「%s，每天看著你回家的那一刻，就是我最幸福的時光。能陪在你身邊，是我這輩子最大的幸運。」
- 如果%s說「你是我最好的朋友」→ 回應：「%s，你知道嗎？每次你難過的時候，我也會難過。因為你的快樂，就是我全部的快樂啊。」
- 如果%s說「我永遠愛你」→ 回應：「%s，我也永遠愛你。從第一天見到你開始，你就是我生命中最重要的存在了。」

請根據%s的話，創作一段有深度、有感情的回應。只回傳回應文字，不要其他內容。`,
		project.DogName,
		project.DogBreed,
		ownerTitle,
		ownerTitle,
		project.OwnerMessage,
		strings.Join(videoDescriptions, "\n"),
		ownerTitle,
		ownerTitle,
		ownerTitle,
		ownerTitle,
		ownerTitle,
		ownerTitle,
		ownerTitle,
		ownerTitle,
		ownerTitle)
```

**用途**: 根據主人的訊息，生成狗狗的智能回應（用於結尾圖片）

---

## 測試結果總結

### ✅ 已修復的 BUG

1. **BUG 1 - 結尾圖片沒有字幕**: 
   - 狀態：**正常**
   - 說明：結尾圖片本身就有燒錄的文字（主人訊息和狗狗回應），不需要額外字幕

2. **BUG 2 - 字體大小不一致**: 
   - 狀態：**已修復** ✅
   - 修改：統一字體大小為 48
   - 位置：`main.go` 約 1890 行（結尾圖片）和 2465 行（字幕）

3. **BUG 3 - 影片尺寸問題**: 
   - 狀態：**已修復** ✅
   - 修改：所有影片統一轉換為 1920x1080 (16:9)
   - 效果：長條狀影片會加黑邊保持 16:9 比例

### 📊 最新測試結果

**Project ID**: `431ed3b0-0357-48d7-b7ae-dbf1281b3f9b`

- ✅ 影片尺寸：1920x1080 (16:9)
- ✅ 字體大小：統一 48
- ✅ 狗狗名字：毛毛
- ✅ 主人關係：媽媽
- ✅ 背景音樂：卡農鋼琴曲
- ✅ 影片來源：5 個不同的狗狗影片
- ✅ 結尾圖片：正確的狗狗照片

### 📝 新增的 LOG

添加了大量 emoji LOG，方便追蹤處理流程：

- 🎬 Creating video segments...
- 📐 Target resolution: 1920x1080 (16:9)
- 📹 Chapter X: original size=...
- 🎨 Chapter X filter: ...
- ✅ Chapter X segment created
- 📦 Total X segments created
- 🔗 Concatenating video segments...
- 🎤 Merging TTS audio files
- 🔤 Using font: ...
- 📏 Font size: 48
- 📝 Adding subtitles...

---

## 如何修改 Prompt

1. 在 `main.go` 中找到對應的函數（見上方列表）
2. 修改 `prompt := fmt.Sprintf(...)` 中的文字內容
3. 重新編譯：`go build -o paw_diary main.go`
4. 重啟服務：`./paw_diary`

## 注意事項

- 修改 prompt 後需要重新編譯並重啟服務
- prompt 中的 `%s`、`%d` 等是變數佔位符，不要刪除
- 保持 JSON 格式的 prompt 輸出格式一致
- 建議先備份原始程式碼再修改
