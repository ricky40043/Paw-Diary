#!/bin/bash

# 測試雙模型設定

API_KEY="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"

echo "=== 測試視覺模型: google/gemini-2.0-flash-001 ==="
curl -s https://openrouter.ai/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "google/gemini-2.0-flash-001",
    "messages": [{"role": "user", "content": "Say hi"}]
  }'

echo ""
echo ""
echo "=== 測試文字模型: openai/gpt-5-nano ==="
curl -s https://openrouter.ai/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "openai/gpt-5-nano",
    "messages": [{"role": "user", "content": "Say hi in Chinese"}]
  }'
