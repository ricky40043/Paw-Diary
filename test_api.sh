#!/bin/bash
source .env
echo "Testing with :free suffix..."
curl -s -X POST $AI_API_ENDPOINT \
  -H "Authorization: Bearer $AI_API_KEY" \
  -H "Content-Type: application/json" \
  -H "HTTP-Referer: https://pawdiary.app" \
  -d '{
    "model": "qwen/qwen-2.5-vl-7b-instruct:free",
    "messages": [
      {"role": "user", "content": "Hello, are you working?"}
    ]
  }' | head -n 20

echo -e "\n\nTesting WITHOUT :free suffix..."
curl -s -X POST $AI_API_ENDPOINT \
  -H "Authorization: Bearer $AI_API_KEY" \
  -H "Content-Type: application/json" \
  -H "HTTP-Referer: https://pawdiary.app" \
  -d '{
    "model": "qwen/qwen-2.5-vl-7b-instruct",
    "messages": [
      {"role": "user", "content": "Hello, are you working?"}
    ]
  }' | head -n 20
