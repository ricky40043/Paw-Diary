#!/bin/bash

# 測試上傳影片腳本

API_URL="http://localhost:8080/api/v1/poc/jobs"

if [ -z "$1" ]; then
    echo "使用方式: bash test_upload.sh <video_file_path>"
    echo "範例: bash test_upload.sh test_video.mp4"
    exit 1
fi

VIDEO_FILE="$1"

if [ ! -f "$VIDEO_FILE" ]; then
    echo "錯誤：找不到檔案 $VIDEO_FILE"
    exit 1
fi

echo "=========================================="
echo "測試上傳影片到 API"
echo "=========================================="
echo "檔案: $VIDEO_FILE"
echo "API: $API_URL"
echo ""

# 上傳影片
echo "上傳中..."
RESPONSE=$(curl -s -X POST "$API_URL" \
  -F "file=@$VIDEO_FILE")

echo "回應："
echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"
echo ""

# 提取 job_id
JOB_ID=$(echo "$RESPONSE" | jq -r '.job_id' 2>/dev/null)

if [ -z "$JOB_ID" ] || [ "$JOB_ID" = "null" ]; then
    echo "上傳失敗！"
    exit 1
fi

echo "任務已建立！Job ID: $JOB_ID"
echo ""
echo "查詢任務狀態中..."

# 輪詢任務狀態
for i in {1..30}; do
    sleep 2
    STATUS_RESPONSE=$(curl -s "http://localhost:8080/api/v1/poc/jobs/$JOB_ID")
    STATUS=$(echo "$STATUS_RESPONSE" | jq -r '.status' 2>/dev/null)
    
    echo "[$i] 狀態: $STATUS"
    
    if [ "$STATUS" = "completed" ]; then
        echo ""
        echo "=========================================="
        echo "處理完成！"
        echo "=========================================="
        echo "$STATUS_RESPONSE" | jq . 2>/dev/null || echo "$STATUS_RESPONSE"
        echo ""
        echo "查看結果: http://localhost:8080/poc/jobs/$JOB_ID"
        exit 0
    elif [ "$STATUS" = "failed" ]; then
        echo ""
        echo "=========================================="
        echo "處理失敗！"
        echo "=========================================="
        echo "$STATUS_RESPONSE" | jq . 2>/dev/null || echo "$STATUS_RESPONSE"
        exit 1
    fi
done

echo ""
echo "輪詢超時，請手動檢查: http://localhost:8080/poc/jobs/$JOB_ID"
