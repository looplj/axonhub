#!/bin/bash

# 翻译检查脚本
# 检查前端代码中使用的翻译 key 和翻译文件中的 key 是否一致

set -e

LOCALE_DIR="$(dirname "$0")"
FRONTEND_DIR="$(dirname "$LOCALE_DIR")"
EN_DIR="$LOCALE_DIR/en"
ZH_DIR="$LOCALE_DIR/zh-CN"

echo "=== 翻译检查脚本 ==="
echo "前端代码目录: $FRONTEND_DIR"
echo "翻译文件目录: $LOCALE_DIR"
echo ""

# 临时文件
USED_KEYS_FILE=$(mktemp)
ALL_LOCALE_KEYS_FILE=$(mktemp)

# 1. 提取前端代码中使用的翻译 key
echo "步骤 1: 扫描前端代码中使用的翻译 key..."

# 匹配 t('key') 或 t("key") 的模式
# 排除注释行和字符串中的内容
# 只匹配有效的翻译 key（排除路径、选择器、纯符号等）
find "$FRONTEND_DIR" -type f \( -name "*.tsx" -o -name "*.ts" -o -name "*.jsx" -o -name "*.js" \) \
  -not -path "*/node_modules/*" \
  -not -path "*/.next/*" \
  -not -path "*/dist/*" \
  -not -path "*/build/*" \
  -exec grep -hE "t\(['\"]([^'\"]+)['\"]" {} \; 2>/dev/null | \
  sed -E "s/.*t\(['\"]([^'\"]+)['\"].*/\1/" | \
  grep -E '[a-zA-Z]' | \
  grep -vE '^/' | \
  grep -vE '^\[' | \
  grep -vE '\.$' | \
  grep -vE '^[0-9]+$' | \
  grep -vE '^(button|form|input|loading|message|text|sidebar_state|content-type|en-US|2d|copy)$' | \
  grep -vE '\\n' | \
  sort -u > "$USED_KEYS_FILE"

USED_KEYS_COUNT=$(wc -l < "$USED_KEYS_FILE")
echo "找到 $USED_KEYS_COUNT 个使用的翻译 key"
echo ""

# 2. 提取所有翻译文件中的 key
echo "步骤 2: 扫描翻译文件中的所有 key..."

# 使用 jq 递归提取所有 key（完整路径）
extract_json_keys() {
  local file="$1"
  # 使用 jq 递归提取所有叶子节点的路径
  jq -r 'paths(scalars) | join(".")' "$file" 2>/dev/null | sort -u
}

# 合并所有英文翻译文件的 key
for en_file in "$EN_DIR"/*.json; do
  if [ -f "$en_file" ]; then
    extract_json_keys "$en_file" >> "$ALL_LOCALE_KEYS_FILE"
  fi
done

# 去重
sort -u "$ALL_LOCALE_KEYS_FILE" -o "$ALL_LOCALE_KEYS_FILE"

LOCALE_KEYS_COUNT=$(wc -l < "$ALL_LOCALE_KEYS_FILE")
echo "找到 $LOCALE_KEYS_COUNT 个翻译文件中的 key"
echo ""

# 3. 对比并找出缺失和多余的 key
echo "步骤 3: 对比分析..."

# 代码中使用但翻译文件中不存在的 key（缺失的翻译）
MISSING_KEYS_FILE=$(mktemp)
comm -23 "$USED_KEYS_FILE" "$ALL_LOCALE_KEYS_FILE" > "$MISSING_KEYS_FILE"
MISSING_KEYS_COUNT=$(wc -l < "$MISSING_KEYS_FILE")

# 翻译文件中存在但代码中未使用的 key（多余的翻译）
UNUSED_KEYS_FILE=$(mktemp)
comm -13 "$USED_KEYS_FILE" "$ALL_LOCALE_KEYS_FILE" > "$UNUSED_KEYS_FILE"
UNUSED_KEYS_COUNT=$(wc -l < "$UNUSED_KEYS_FILE")

echo ""

# 4. 输出结果
echo "=== 检查结果 ==="
echo ""

if [ "$MISSING_KEYS_COUNT" -gt 0 ]; then
  echo "❌ 缺失的翻译 key（代码中使用但翻译文件中不存在）: $MISSING_KEYS_COUNT 个"
  echo ""
  cat "$MISSING_KEYS_FILE" | sed 's/^/  - /'
  echo ""
else
  echo "✅ 没有缺失的翻译 key"
  echo ""
fi

if [ "$UNUSED_KEYS_COUNT" -gt 0 ]; then
  echo "⚠️  多余的翻译 key（翻译文件中存在但代码中未使用）: $UNUSED_KEYS_COUNT 个"
  echo ""
  cat "$UNUSED_KEYS_FILE" | sed 's/^/  - /'
  echo ""
else
  echo "✅ 没有多余的翻译 key"
  echo ""
fi

echo "=== 汇总 ==="
echo "代码中使用的翻译 key: $USED_KEYS_COUNT 个"
echo "翻译文件中的 key: $LOCALE_KEYS_COUNT 个"
echo "缺失的翻译 key: $MISSING_KEYS_COUNT 个"
echo "多余的翻译 key: $UNUSED_KEYS_COUNT 个"
echo ""

# 清理临时文件
rm -f "$USED_KEYS_FILE" "$ALL_LOCALE_KEYS_FILE" "$MISSING_KEYS_FILE" "$UNUSED_KEYS_FILE"

# 返回退出码
if [ "$MISSING_KEYS_COUNT" -gt 0 ]; then
  echo "❌ 检查失败：存在缺失的翻译 key"
  exit 1
else
  echo "✅ 检查通过：所有使用的翻译 key 都已定义"
  exit 0
fi
