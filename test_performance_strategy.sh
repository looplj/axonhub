#!/bin/bash
# Test script to verify performance strategy is working

# Set your API key and base URL
API_KEY="your-api-key-here"
BASE_URL="http://localhost:8090/v1"

# 1. Set the performance strategy in system settings
echo "Setting performance strategy..."
curl -X POST "${BASE_URL}/api/system/retry-policy" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "loadBalancerStrategy": "performance",
    "maxChannelRetries": 3,
    "maxSingleChannelRetries": 2,
    "retryDelayMs": 1000
  }'

echo -e "\n\n2. Making a test request to verify routing..."
# 2. Make a test request
curl -X POST "${BASE_URL}/chat/completions" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello, how are you?"}],
    "stream": false
  }'

echo -e "\n\nCheck the server logs for: load_balance_strategy=performance"
