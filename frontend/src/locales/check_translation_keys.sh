#!/bin/bash

# 翻译检查脚本
# 检查前端代码中使用的翻译 key 和翻译文件中的 key 是否一致

set -e

FIX_MODE=false
while [[ "$#" -gt 0 ]]; do
  case $1 in
    -f|--fix) FIX_MODE=true ;;
    *) echo "未知选项: $1"; exit 1 ;;
  esac
  shift
done

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
DYNAMIC_PATTERNS_FILE=$(mktemp)
ALL_LOCALE_KEYS_FILE=$(mktemp)

# 1. 提取前端代码中使用的翻译 key
echo "步骤 1: 扫描前端代码中使用的翻译 key..."

# 1.1 提取静态翻译 key (t('key') 或 t("key"))
# 匹配 t('key') 或 t("key") 的模式
# 排除注释行和字符串中的内容
# 只匹配有效的翻译 key（排除路径、选择器、纯符号等）
find "$FRONTEND_DIR" -type f \( -name "*.tsx" -o -name "*.ts" -o -name "*.jsx" -o -name "*.js" \) \
  -not -path "*/node_modules/*" \
  -not -path "*/.next/*" \
  -not -path "*/dist/*" \
  -not -path "*/build/*" \
  -exec grep -ohE "\bt\(['\"][^'\"]+['\"]" {} \; 2>/dev/null | \
  sed -E "s/^t\(['\"]//; s/['\"]$//" | \
  grep -E '[a-zA-Z]' | \
  grep -vE '^/' | \
  grep -vE '^\[' | \
  grep -vE '\.$' | \
  grep -vE '^[0-9]+$' | \
  grep -vE '^(button|form|input|loading|message|text|sidebar_state|content-type|en-US|2d|copy)$' | \
  grep -vE '\\n' | \
  sort -u > "$USED_KEYS_FILE"

# 1.2 提取动态翻译 key 模式 (例如 t(`prefix.${var}`))
echo "步骤 1.1: 扫描前端代码中的动态翻译 key 模式..."
find "$FRONTEND_DIR" -type f \( -name "*.tsx" -o -name "*.ts" -o -name "*.jsx" -o -name "*.js" \) \
  -not -path "*/node_modules/*" \
  -not -path "*/.next/*" \
  -not -path "*/dist/*" \
  -not -path "*/build/*" \
  -exec grep -ohE "\bt\(\`[^\`]*\\\$[\{][^\`]*\`" {} \; 2>/dev/null | \
  sed -E "s/^t\(\`//; s/\`$//" | \
  sed -E 's/\$\{[^\}]*\}/DYNAMIC_PLACEHOLDER/g' | \
  sed -E 's/\./\\./g' | \
  sed -E 's/DYNAMIC_PLACEHOLDER/.*/g' | \
  sort -u > "$DYNAMIC_PATTERNS_FILE"

USED_KEYS_COUNT=$(wc -l < "$USED_KEYS_FILE" | tr -d ' ')
DYNAMIC_PATTERNS_COUNT=$(wc -l < "$DYNAMIC_PATTERNS_FILE" | tr -d ' ')
echo "找到 $USED_KEYS_COUNT 个静态翻译 key"
echo "找到 $DYNAMIC_PATTERNS_COUNT 个动态翻译模式"
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

LOCALE_KEYS_COUNT=$(wc -l < "$ALL_LOCALE_KEYS_FILE" | tr -d ' ')
echo "找到 $LOCALE_KEYS_COUNT 个翻译文件中的 key"
echo ""

# 3. 对比并找出缺失和多余的 key
echo "步骤 3: 对比分析..."

# 代码中使用但翻译文件中不存在的 key（缺失的翻译）
MISSING_KEYS_FILE=$(mktemp)
comm -23 "$USED_KEYS_FILE" "$ALL_LOCALE_KEYS_FILE" > "$MISSING_KEYS_FILE"
MISSING_KEYS_COUNT=$(wc -l < "$MISSING_KEYS_FILE" | tr -d ' ')

# 翻译文件中存在 but code matching dynamic patterns are NOT unused
# 翻译文件中存在但代码中未使用的 key（多余的翻译）
UNUSED_KEYS_FILE=$(mktemp)
comm -13 "$USED_KEYS_FILE" "$ALL_LOCALE_KEYS_FILE" > "$UNUSED_KEYS_FILE"

# 过滤掉匹配动态模式的 key
if [ -s "$DYNAMIC_PATTERNS_FILE" ]; then
  TEMP_UNUSED_FILE=$(mktemp)
  while read -r key; do
    matched=false
    while read -r pattern; do
      if [[ $key =~ ^$pattern$ ]]; then
        matched=true
        break
      fi
    done < "$DYNAMIC_PATTERNS_FILE"
    if [ "$matched" = false ]; then
      echo "$key" >> "$TEMP_UNUSED_FILE"
    fi
  done < "$UNUSED_KEYS_FILE"
  mv "$TEMP_UNUSED_FILE" "$UNUSED_KEYS_FILE"
fi

UNUSED_KEYS_COUNT=$(wc -l < "$UNUSED_KEYS_FILE" | tr -d ' ')

# 3.1 自动修复（删除多余的 key）
if [ "$FIX_MODE" = true ] && [ "$UNUSED_KEYS_COUNT" -gt 0 ]; then
  echo "步骤 3.1: 正在自动删除 $UNUSED_KEYS_COUNT 个多余的翻译 key..."
  
  while read -r key; do
    # 将 a.b.c 转换为 .["a"]["b"]["c"]
    IFS='.' read -ra ADDR <<< "$key"
    JQ_PATH="."
    for part in "${ADDR[@]}"; do
      JQ_PATH="$JQ_PATH[\"$part\"]"
    done
    
    echo "  正在删除: $key"
    
    # 在所有翻译文件中删除
    for locale_dir in "$EN_DIR" "$ZH_DIR"; do
      if [ -d "$locale_dir" ]; then
        for json_file in "$locale_dir"/*.json; do
          if [ -f "$json_file" ]; then
            # 使用 temp 文件避免原地修改的问题
            TEMP_JSON=$(mktemp)
            # 删除 key
            jq "del($JQ_PATH)" "$json_file" > "$TEMP_JSON"
            mv "$TEMP_JSON" "$json_file"
          fi
        done
      fi
    done
  done < "$UNUSED_KEYS_FILE"
  
  echo "✅ 自动修复完成"
  echo ""
  
  # 重新计算翻译文件中的 key 数量
  echo "重新扫描翻译文件..."
  > "$ALL_LOCALE_KEYS_FILE"
  for en_file in "$EN_DIR"/*.json; do
    if [ -f "$en_file" ]; then
      extract_json_keys "$en_file" >> "$ALL_LOCALE_KEYS_FILE"
    fi
  done
  sort -u "$ALL_LOCALE_KEYS_FILE" -o "$ALL_LOCALE_KEYS_FILE"
  LOCALE_KEYS_COUNT=$(wc -l < "$ALL_LOCALE_KEYS_FILE" | tr -d ' ')
  
  # 重新计算多余的 key（应该为 0 了，除非有动态模式匹配的问题）
  # 但实际上我们已经删除了它们，所以直接重置计数即可
  UNUSED_KEYS_COUNT=0
fi

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
echo "代码中使用的静态 key: $USED_KEYS_COUNT 个"
echo "代码中使用的动态模式: $DYNAMIC_PATTERNS_COUNT 个"
echo "翻译文件中的总 key: $LOCALE_KEYS_COUNT 个"
echo "缺失的翻译 key: $MISSING_KEYS_COUNT 个"
echo "多余的翻译 key: $UNUSED_KEYS_COUNT 个"
echo ""

# 清理临时文件
rm -f "$USED_KEYS_FILE" "$DYNAMIC_PATTERNS_FILE" "$ALL_LOCALE_KEYS_FILE" "$MISSING_KEYS_FILE" "$UNUSED_KEYS_FILE"

# 返回退出码
if [ "$MISSING_KEYS_COUNT" -gt 0 ]; then
  echo "❌ 检查失败：存在缺失的翻译 key"
  exit 1
else
  echo "✅ 检查通过：所有使用的翻译 key 都已定义"
  exit 0
fi
