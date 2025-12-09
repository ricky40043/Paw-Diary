# 結尾圖片修復說明

## 問題描述
項目 `e0c46709-79b6-487e-a013-112349d643df` 的最後一張圖片顯示異常（藍屏）。

## 根本原因
結尾圖片的顏色空間設置不正確，導致顯示問題：
- **舊版本**: 使用 `yuvj420p` (JPEG 顏色空間，full range)，轉換為 `bt2020nc`
- **新版本**: 使用 `yuv420p` (標準視頻顏色空間，tv range)，明確指定 `bt709`

## 修復內容

### main.go 第 1840-1876 行
在 `addEndingImage` 函數中的 FFmpeg 命令添加了：

1. **顏色空間轉換濾鏡**:
   ```
   format=yuv420p,colorspace=bt709:iall=bt601-6-625:fast=1
   ```

2. **明確指定輸出顏色參數**:
   ```
   -color_range tv
   -colorspace bt709
   -color_primaries bt709
   -color_trc bt709
   ```

3. **Padding 顏色**:
   ```
   pad=....:color=black
   ```
   確保填充區域是純黑色

## 測試結果

### 手動測試（無需 AI）
```bash
# 已測試生成結尾影片片段
/tmp/test_ending_segment.mp4

# 顏色空間驗證
pix_fmt=yuv420p
color_range=tv
color_space=bt709

# 狀態: ✅ 正確
```

## 如何驗證修復

### 方法 1: 重新生成項目（推薦）
需要重新運行 `go run test_v2_flow.go` 來生成新項目，舊項目是用舊代碼生成的。

### 方法 2: 手動測試結尾圖片合成
不需要 AI，只需測試結尾圖片部分：

```bash
# 1. 準備測試文件
cp storage/projects/e0c46709-79b6-487e-a013-112349d643df/ending_image.jpg /tmp/test_img.jpg

# 2. 生成結尾片段（使用新代碼的命令）
ffmpeg -loop 1 -i /tmp/test_img.jpg \
  -f lavfi -i anullsrc=r=44100:cl=stereo \
  -vf "scale=720:1280:force_original_aspect_ratio=decrease,\
       pad=720:1280:(ow-iw)/2:(oh-ih)/2:color=black,\
       drawtext=fontfile='/System/Library/Fonts/STHeiti Medium.ttc':\
       text='🐾 測試：結尾圖片修復測試':\
       fontsize=48:fontcolor=white:\
       x=(w-text_w)/2:y=h-256:\
       box=1:boxcolor=black@0.6:boxborderw=10,\
       fade=t=in:st=0:d=0.5,fade=t=out:st=9.5:d=0.5,\
       format=yuv420p,colorspace=bt709:iall=bt601-6-625:fast=1" \
  -t 10 -c:v libx264 -c:a aac -pix_fmt yuv420p \
  -color_range tv -colorspace bt709 -color_primaries bt709 -color_trc bt709 \
  -shortest -y /tmp/test_ending_new.mp4

# 3. 播放測試
ffplay /tmp/test_ending_new.mp4

# 4. 檢查顏色空間
ffprobe -v error -select_streams v:0 \
  -show_entries stream=pix_fmt,color_range,color_space \
  /tmp/test_ending_new.mp4
```

### 方法 3: 檢查現有影片
```bash
# 提取舊影片的結尾部分
ffmpeg -ss 60 -i storage/projects/e0c46709-79b6-487e-a013-112349d643df/final.mp4 \
  -t 10 -c copy /tmp/old_ending.mp4

# 檢查顏色空間
ffprobe -v error -select_streams v:0 \
  -show_entries stream=pix_fmt,color_range,color_space \
  /tmp/old_ending.mp4

# 結果應該是:
# 舊版: color_space=bt2020nc (可能導致問題)
# 新版: color_space=bt709 (正確)
```

## 對比

| 項目 | 舊版本 | 新版本 |
|------|--------|--------|
| 顏色空間 | bt2020nc | bt709 |
| 顏色範圍 | tv | tv |
| 像素格式 | yuv420p | yuv420p |
| 顏色轉換 | ❌ 無 | ✅ 有 |
| Padding | 默認灰色 | 黑色 |

## 後續步驟

1. ✅ 代碼已修復
2. ✅ 手動測試通過
3. ⏳ 需要完整項目測試（重新運行 test_v2_flow.go）

## 注意事項

- 舊項目（如 e0c46709）是用舊代碼生成的，不會自動修復
- 需要重新生成項目才能看到修復效果
- 手動測試表明新代碼可以正確生成結尾圖片
- 顏色空間現在與主影片一致，應該不會再出現藍屏問題
