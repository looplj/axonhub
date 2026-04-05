#!/bin/bash

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCALES_DIR="$SCRIPT_DIR"
FRONTEND_DIR="$(dirname "$LOCALES_DIR")"

COMMON_COLUMN_KEYS=(
  "id"
  "description"
  "name"
  "status"
  "actions"
  "createdAt"
  "updatedAt"
  "selectAll"
  "selectRow"
)

echo "=========================================="
echo "Step 1: Cleaning locale JSON files"
echo "=========================================="

for lang_dir in "$LOCALES_DIR"/*; do
  if [ -d "$lang_dir" ]; then
    lang=$(basename "$lang_dir")
    
    for json_file in "$lang_dir"/*.json; do
      if [ -f "$json_file" ]; then
        filename=$(basename "$json_file")
        
        if [ "$filename" = "base.json" ]; then
          continue
        fi
        
        echo "Processing $lang_dir/$filename"
        
        temp_file="${json_file}.tmp"
        
        # 扁平化格式：删除所有以 "*.columns.<key>" 形式存在的 key
        # 构建 jq 的 del 表达式
        del_expr="del("
        first=true
        for key in "${COMMON_COLUMN_KEYS[@]}"; do
          if [ "$first" = true ]; then
            first=false
          else
            del_expr="$del_expr, "
          fi
          del_expr="${del_expr}.[\"columns.${key}\"]"
        done
        del_expr="$del_expr)"
        
        jq "$del_expr" "$json_file" > "$temp_file"
        
        mv "$temp_file" "$json_file"
        
        echo "  Cleaned common column keys from $filename"
      fi
    done
  fi
done

echo ""
echo "Done! Common column keys have been removed from all locale files except base.json"

echo ""
echo "=========================================="
echo "Step 2: Replacing old i18n keys in code files"
echo "=========================================="

COMMON_COLUMN_KEYS_PATTERN=$(IFS="|"; echo "${COMMON_COLUMN_KEYS[*]}")

for file in $(find "$FRONTEND_DIR" -type f \( -name "*.tsx" -o -name "*.ts" \)); do
  if [ -f "$file" ]; then
    temp_file="${file}.tmp"
    
    # 适配扁平化格式：匹配任意前缀的 columns.<key> 形式，替换为 common.columns.<key>
    if sed -E "s/t\(['\"])([a-zA-Z0-9_-]+)\.columns\.($COMMON_COLUMN_KEYS_PATTERN)\1/t('common.columns.\2')/g" "$file" > "$temp_file" 2>/dev/null; then
      if ! diff -q "$file" "$temp_file" > /dev/null 2>&1; then
        echo "Updated: $file"
        mv "$temp_file" "$file"
      else
        rm "$temp_file"
      fi
    else
      rm -f "$temp_file"
    fi
  fi
done

echo ""
echo "Done! Old i18n keys have been replaced with common i18n keys in code files"
echo ""
echo "=========================================="
echo "Summary"
echo "=========================================="
echo "1. Removed common column keys from locale JSON files (except base.json)"
echo "2. Replaced old i18n keys (e.g., t('roles.columns.id')) with common keys (e.g., t('common.columns.id')) in TypeScript/TSX files"
echo "=========================================="
