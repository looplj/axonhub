#!/bin/bash

# 翻译文件对比脚本
# 检查中英文翻译是否有遗漏

LOCALE_DIR="$(dirname "$0")"
EN_DIR="$LOCALE_DIR/en"
ZH_DIR="$LOCALE_DIR/zh-CN"

echo "=== 翻译文件对比检查 ==="
echo ""

total_en_missing=0
total_zh_missing=0

for en_file in "$EN_DIR"/*.json; do
  filename=$(basename "$en_file")
  zh_file="$ZH_DIR/$filename"

  if [ ! -f "$zh_file" ]; then
    echo "警告: 缺少中文翻译文件 $filename"
    continue
  fi

  # 提取并排序 key
  en_keys=$(jq -r 'keys[]' "$en_file" 2>/dev/null | sort)
  zh_keys=$(jq -r 'keys[]' "$zh_file" 2>/dev/null | sort)

  # 英文有，中文缺失
  en_only=$(comm -23 <(echo "$en_keys") <(echo "$zh_keys"))
  # 中文有，英文缺失
  zh_only=$(comm -23 <(echo "$zh_keys") <(echo "$en_keys"))

  en_count=$(echo "$en_only" | grep -c .)
  zh_count=$(echo "$zh_only" | grep -c .)

  if [ -n "$en_only" ] || [ -n "$zh_only" ]; then
    echo "--- $filename ---"
    if [ -n "$en_only" ]; then
      echo "英文有，中文缺失 ($en_count 个):"
      echo "$en_only" | sed 's/^/  - /'
    fi
    if [ -n "$zh_only" ]; then
      echo "中文有，英文缺失 ($zh_count 个):"
      echo "$zh_only" | sed 's/^/  - /'
    fi
    echo ""
  fi

  total_en_missing=$((total_en_missing + en_count))
  total_zh_missing=$((total_zh_missing + zh_count))
done

echo "=== 汇总 ==="
echo "英文有，中文缺失: $total_en_missing 个"
echo "中文有，英文缺失: $total_zh_missing 个"
