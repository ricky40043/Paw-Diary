#!/bin/bash

# Phase 2 測試腳本 - 多影片故事生成

echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║   🎬 Phase 2 測試 - 多影片故事生成                            ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
echo ""

API_URL="http://localhost:8080"

# 檢查是否有影片檔案
if [ ! -f "786462251.824761.mp4" ]; then
    echo "❌ 找不到測試影片 786462251.824761.mp4"
    echo "請提供至少一個影片檔案"
    exit 1
fi

echo "=== Step 1: 建立專案 ==="
PROJECT_RESPONSE=$(curl -s -X POST "$API_URL/api/v2/story/projects" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "豆豆的溫馨時光",
    "dog_name": "豆豆",
    "dog_breed": "吉娃娃"
  }')

echo "$PROJECT_RESPONSE" | jq .

PROJECT_ID=$(echo "$PROJECT_RESPONSE" | jq -r '.project_id')

if [ -z "$PROJECT_ID" ] || [ "$PROJECT_ID" = "null" ]; then
    echo "❌ 建立專案失敗"
    exit 1
fi

echo ""
echo "✅ 專案已建立：$PROJECT_ID"
echo ""

echo "=== Step 2: 上傳影片 ==="
UPLOAD_RESPONSE=$(curl -s -X POST "$API_URL/api/v2/story/projects/$PROJECT_ID/videos" \
  -F "videos=@786462251.824761.mp4")

echo "$UPLOAD_RESPONSE" | jq .

UPLOADED_COUNT=$(echo "$UPLOAD_RESPONSE" | jq -r '.uploaded')

if [ "$UPLOADED_COUNT" = "0" ] || [ "$UPLOADED_COUNT" = "null" ]; then
    echo "❌ 上傳影片失敗"
    exit 1
fi

echo ""
echo "✅ 已上傳 $UPLOADED_COUNT 個影片"
echo ""

echo "=== Step 3: 生成故事 ==="
GENERATE_RESPONSE=$(curl -s -X POST "$API_URL/api/v2/story/projects/$PROJECT_ID/generate")

echo "$GENERATE_RESPONSE" | jq .
echo ""

echo "=== Step 4: 輪詢處理狀態 ==="
echo "處理中，請稍候..."
echo ""

for i in {1..60}; do
    sleep 3
    STATUS_RESPONSE=$(curl -s "$API_URL/api/v2/story/projects/$PROJECT_ID")
    STATUS=$(echo "$STATUS_RESPONSE" | jq -r '.status')
    
    echo "[$i] 狀態: $STATUS"
    
    if [ "$STATUS" = "completed" ]; then
        echo ""
        echo "╔═══════════════════════════════════════════════════════════════╗"
        echo "║   ✅ 處理完成！                                                ║"
        echo "╚═══════════════════════════════════════════════════════════════╝"
        echo ""
        
        # 顯示故事資訊
        echo "=== 故事資訊 ==="
        TITLE=$(echo "$STATUS_RESPONSE" | jq -r '.story.title')
        CHAPTERS=$(echo "$STATUS_RESPONSE" | jq '.story.chapters | length')
        
        echo "標題：$TITLE"
        echo "章節數：$CHAPTERS"
        echo ""
        
        echo "=== 故事章節 ==="
        echo "$STATUS_RESPONSE" | jq -r '.story.chapters[] | "[\(.index)] \(.narration)"'
        echo ""
        
        echo "=== 最終影片 ==="
        FINAL_VIDEO_URL=$(echo "$STATUS_RESPONSE" | jq -r '.final_video_url')
        echo "影片 URL: $API_URL$FINAL_VIDEO_URL"
        echo ""
        
        echo "下載影片："
        echo "  curl -O $API_URL$FINAL_VIDEO_URL"
        echo ""
        
        exit 0
    elif [ "$STATUS" = "failed" ]; then
        echo ""
        echo "❌ 處理失敗"
        ERROR=$(echo "$STATUS_RESPONSE" | jq -r '.error')
        echo "錯誤訊息：$ERROR"
        exit 1
    fi
done

echo ""
echo "⏰ 處理超時，請手動檢查："
echo "  curl $API_URL/api/v2/story/projects/$PROJECT_ID | jq ."
