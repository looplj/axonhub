#!/bin/bash

LOCALES_DIR="$(dirname "$0")"
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
        
        jq 'walk(if type == "object" and has("columns") then 
          .columns |= del(.id, .name, .status, .actions, .createdAt, .updatedAt, .selectAll, .selectRow)
        else 
          .
        end)' "$json_file" > "$temp_file"
        
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
    
    if sed -E "s/t\('([a-zA-Z0-9_]+)\.columns\.($COMMON_COLUMN_KEYS_PATTERN)'\)/t('common.columns.\2')/g" "$file" > "$temp_file" 2>/dev/null; then
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
