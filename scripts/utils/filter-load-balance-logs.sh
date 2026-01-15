#!/bin/bash

# Filter and analyze load balance logs
# Usage: ./scripts/utils/filter-load-balance-logs.sh [options] [log_file]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
LOG_FILE=""
SINCE=""
UNTIL=""
CHANNEL_ID=""
MODEL=""
MIN_SCORE=""
MAX_SCORE=""
MIN_RANK=""
MAX_RANK=""
SHOW_SUMMARY=false
SHOW_STATS=false
SHOW_DETAILS=false
STRATEGY=""
SHOW_DECISION_ONLY=false
SHOW_CHANNEL_ONLY=false
LIMIT=1000
OUTPUT_FORMAT="table"

# Print usage
usage() {
    cat << EOF
Usage: $(basename "$0") [options] [log_file]

Filter and analyze load balance logs from server output.

Arguments:
  log_file              Path to log file (default: reads from stdin)

Options:
  --since TIME          Show logs since TIME (e.g., "10m", "1h", "2024-01-01T10:00:00")
  --until TIME          Show logs until TIME
  --channel-id ID       Filter by channel ID
  --model MODEL         Filter by model name
  --min-score SCORE     Filter by minimum total score
  --max-score SCORE     Filter by maximum total score
  --min-rank RANK       Filter by minimum rank
  --max-rank RANK       Filter by maximum rank
  --strategy NAME       Filter by strategy name
  --summary             Show summary of load balancing decisions
  --stats               Show statistics (channel selection count, average scores, etc.)
  --details             Show detailed strategy breakdown
  --decision-only       Show only decision logs (not channel details)
  --channel-only        Show only channel details (not decision logs)
  --limit N             Limit output to N entries (default: 100)
  --format FORMAT       Output format: table, json, csv (default: table)
  -h, --help            Show this help message

Examples:
  # Show last 10 minutes of load balance logs
  $(basename "$0") --since 10m server.log

  # Filter by channel ID
  $(basename "$0") --channel-id 1 server.log

  # Show statistics for a specific model
  $(basename "$0") --model gpt-4 --stats server.log

  # Show detailed strategy breakdown
  $(basename "$0") --details --limit 5 server.log

  # Show summary of decisions in the last hour
  $(basename "$0") --since 1h --summary server.log

  # Export to CSV
  $(basename "$0") --format csv --limit 1000 server.log > output.csv

  # Filter by score range
  $(basename "$0") --min-score 1000 --max-score 2000 server.log

  # Show top-ranked channels only
  $(basename "$0") --max-rank 1 server.log

EOF
    exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --since)
            SINCE="$2"
            shift 2
            ;;
        --until)
            UNTIL="$2"
            shift 2
            ;;
        --channel-id)
            CHANNEL_ID="$2"
            shift 2
            ;;
        --model)
            MODEL="$2"
            shift 2
            ;;
        --min-score)
            MIN_SCORE="$2"
            shift 2
            ;;
        --max-score)
            MAX_SCORE="$2"
            shift 2
            ;;
        --min-rank)
            MIN_RANK="$2"
            shift 2
            ;;
        --max-rank)
            MAX_RANK="$2"
            shift 2
            ;;
        --strategy)
            STRATEGY="$2"
            shift 2
            ;;
        --summary)
            SHOW_SUMMARY=true
            shift
            ;;
        --stats)
            SHOW_STATS=true
            shift
            ;;
        --details)
            SHOW_DETAILS=true
            shift
            ;;
        --decision-only)
            SHOW_DECISION_ONLY=true
            shift
            ;;
        --channel-only)
            SHOW_CHANNEL_ONLY=true
            shift
            ;;
        --limit)
            LIMIT="$2"
            shift 2
            ;;
        --format)
            OUTPUT_FORMAT="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            if [[ -z "$LOG_FILE" ]]; then
                LOG_FILE="$1"
            else
                echo -e "${RED}Error: Unknown argument: $1${NC}" >&2
                usage
            fi
            shift
            ;;
    esac
done

# Read from file or stdin
if [[ -n "$LOG_FILE" ]]; then
    if [[ ! -f "$LOG_FILE" ]]; then
        echo -e "${RED}Error: Log file not found: $LOG_FILE${NC}" >&2
        exit 1
    fi
    LOG_CONTENT=$(cat "$LOG_FILE")
else
    LOG_CONTENT=$(cat)
fi

# Filter by time range
filter_by_time() {
    local input="$1"
    local output="$input"

    if [[ -n "$SINCE" ]]; then
        output=$(echo "$output" | awk -v since="$SINCE" '
            {
                match($0, /([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2})/, arr)
                if (arr[1] != "") {
                    if (arr[1] >= since) print $0
                }
            }
        ')
    fi

    if [[ -n "$UNTIL" ]]; then
        output=$(echo "$output" | awk -v until="$UNTIL" '
            {
                match($0, /([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2})/, arr)
                if (arr[1] != "") {
                    if (arr[1] <= until) print $0
                }
            }
        ')
    fi

    echo "$output"
}

# Filter load balance logs
filter_load_balance_logs() {
    local input="$1"
    local output

    if [[ "$SHOW_DECISION_ONLY" == true ]]; then
        output=$(echo "$input" | grep "Load balancing decision completed")
    elif [[ "$SHOW_CHANNEL_ONLY" == true ]]; then
        output=$(echo "$input" | grep "Channel load balancing details")
    else
        output=$(echo "$input" | grep -E "(Load balancing decision completed|Channel load balancing details)")
    fi

    # Apply filters
    if [[ -n "$CHANNEL_ID" ]]; then
        output=$(echo "$output" | grep "channel_id=$CHANNEL_ID")
    fi

    if [[ -n "$MODEL" ]]; then
        output=$(echo "$output" | grep "model=$MODEL")
    fi

    if [[ -n "$MIN_SCORE" ]]; then
        output=$(echo "$output" | awk -v min="$MIN_SCORE" '
            /total_score=/ {
                match($0, /total_score=([0-9.]+)/, arr)
                if (arr[1] >= min) print $0
            }
            !/total_score=/ { print $0 }
        ')
    fi

    if [[ -n "$MAX_SCORE" ]]; then
        output=$(echo "$output" | awk -v max="$MAX_SCORE" '
            /total_score=/ {
                match($0, /total_score=([0-9.]+)/, arr)
                if (arr[1] <= max) print $0
            }
            !/total_score=/ { print $0 }
        ')
    fi

    if [[ -n "$MIN_RANK" ]]; then
        output=$(echo "$output" | awk -v min="$MIN_RANK" '
            /final_rank=/ {
                match($0, /final_rank=([0-9]+)/, arr)
                if (arr[1] >= min) print $0
            }
            !/final_rank=/ { print $0 }
        ')
    fi

    if [[ -n "$MAX_RANK" ]]; then
        output=$(echo "$output" | awk -v max="$MAX_RANK" '
            /final_rank=/ {
                match($0, /final_rank=([0-9]+)/, arr)
                if (arr[1] <= max) print $0
            }
            !/final_rank=/ { print $0 }
        ')
    fi

    if [[ -n "$STRATEGY" ]]; then
        output=$(echo "$output" | grep "$STRATEGY")
    fi

    # Limit output
    if [[ -n "$LIMIT" ]]; then
        output=$(echo "$output" | head -n "$LIMIT")
    fi

    echo "$output"
}

# Extract value using jq (JSON parsing)
extract_value() {
    local line="$1"
    local key="$2"
    echo "$line" | jq -r ".$key // empty"
}

# Show summary of decisions
show_summary() {
    local input="$1"
    local decisions=$(echo "$input" | grep "Load balancing decision completed")

    echo -e "${BLUE}=== Load Balancing Summary ===${NC}"
    echo ""

    local total_decisions=$(echo "$decisions" | wc -l | tr -d ' ')
    echo "Total decisions: $total_decisions"
    echo ""

    # Extract and display unique models
    echo "Models used:"
    echo "$decisions" | jq -r '.model' | sort | uniq -c | sort -rn | sed -E 's/^[[:space:]]*([0-9]+)[[:space:]]+(.*)/  \2: \1/'
    echo ""

    # Extract and display channel selection statistics
    echo "Channel selection count:"
    echo "$decisions" | jq -r '.top_channel_name' | sort | uniq -c | sort -rn | sed -E 's/^[[:space:]]*([0-9]+)[[:space:]]+(.*)/  \2: \1/'
    echo ""

    # Average duration
    local avg_duration=$(echo "$decisions" | jq -r '.duration' | awk '{sum+=$1; count++} END {if (count>0) printf "%.3fs", sum/count}')
    echo "Average duration: $avg_duration"
}

# Show statistics
show_stats() {
    local input="$1"
    local channel_details=$(echo "$input" | grep "Channel load balancing details")

    echo -e "${BLUE}=== Load Balancing Statistics ===${NC}"
    echo ""

    # Channel selection count
    echo "Channel selection count:"
    echo "$channel_details" | jq -r '"\(.channel_id) | \(.channel_name)"' | sort | uniq -c | sort -rn | sed -E 's/^[[:space:]]*([0-9]+)[[:space:]]+([0-9]+)[[:space:]]+\|[[:space:]]+(.*)/  \3 (ID: \2): \1/'
    echo ""

    # Average score per channel
    echo "Average score per channel:"
    echo "$channel_details" | jq -r '"\(.channel_id)|\(.channel_name)|\(.total_score)"' | awk -F'|' '{
        sum[$1] += $3
        count[$1]++
        name[$1] = $2
    }
    END {
        for (id in sum) {
            printf "%.2f|%s|%s|%d\n", sum[id]/count[id], name[id], id, count[id]
        }
    }' | sort -rn | awk -F'|' '{printf "  %s (ID: %s): %s (%d samples)\n", $2, $3, $1, $4}'
    echo ""

    # Rank distribution
    echo "Rank distribution:"
    echo "$channel_details" | jq -r '.final_rank' | sort | uniq -c | awk '{printf "  Rank %s: %d\n", $2, $1}'
    echo ""

    # Strategy statistics
    echo "Strategy usage:"
    echo "$channel_details" | jq -r '.strategy_breakdown | keys[]' | sort | uniq -c | sort -rn | sed -E 's/^[[:space:]]*([0-9]+)[[:space:]]+(.*)/  \2: \1/'
}

# Show detailed strategy breakdown
show_details() {
    local input="$1"
    local channel_details=$(echo "$input" | grep "Channel load balancing details")

    echo -e "${BLUE}=== Detailed Strategy Breakdown ===${NC}"
    echo ""

    echo "$channel_details" | while IFS= read -r line; do
        local channel_id=$(extract_value "$line" 'channel_id')
        local channel_name=$(extract_value "$line" 'channel_name')
        local total_score=$(extract_value "$line" 'total_score')
        local final_rank=$(extract_value "$line" 'final_rank')

        echo -e "${GREEN}Channel: $channel_name (ID: $channel_id)${NC}"
        echo "  Total Score: $total_score"
        echo "  Rank: $final_rank"
        echo "  Strategy Breakdown:"

        # Extract strategy scores using jq
        # Duration is in nanoseconds, converting to milliseconds for better readability
        echo "$line" | jq -r '.strategy_breakdown | to_entries[] | "    - \(.key): \(.value.score) points (\((.value.duration / 1000000.0 * 1000 | round) / 1000)ms)"'
        echo ""
    done
}

# Format output as table
format_as_table() {
    local input="$1"
    
    echo "$input" | while IFS= read -r line; do
        if [[ "$line" =~ "Load balancing decision completed" ]]; then
            local timestamp=$(extract_value "$line" 'time')
            local total_channels=$(extract_value "$line" 'total_channels')
            local selected_channels=$(extract_value "$line" 'selected_channels')
            local top_channel_id=$(extract_value "$line" 'top_channel_id')
            local top_channel_name=$(extract_value "$line" 'top_channel_name')
            local top_channel_score=$(extract_value "$line" 'top_channel_score')
            local model=$(extract_value "$line" 'model')
            local duration=$(extract_value "$line" 'duration')

            echo -e "${YELLOW}=== Decision ===${NC}"
            echo "Timestamp: $timestamp"
            echo "Model: $model"
            echo "Channels: $total_channels total, $selected_channels selected"
            echo "Top Channel: $top_channel_name (ID: $top_channel_id, Score: $top_channel_score)"
            echo "Duration: ${duration}s"
            echo ""
        elif [[ "$line" =~ "Channel load balancing details" ]]; then
            local channel_id=$(extract_value "$line" 'channel_id')
            local channel_name=$(extract_value "$line" 'channel_name')
            local total_score=$(extract_value "$line" 'total_score')
            local final_rank=$(extract_value "$line" 'final_rank')

            echo -e "${GREEN}  Channel: $channel_name (ID: $channel_id)${NC}"
            echo "    Score: $total_score, Rank: $final_rank"
        fi
    done
}

# Format output as JSON
format_as_json() {
    local input="$1"
    
    echo "["
    local first=true
    
    echo "$input" | while IFS= read -r line; do
        [[ "$first" == false ]] && echo ","
        first=false
        
        if [[ "$line" =~ "Load balancing decision completed" ]]; then
            local timestamp=$(extract_value "$line" 'time')
            local total_channels=$(extract_value "$line" 'total_channels')
            local selected_channels=$(extract_value "$line" 'selected_channels')
            local top_channel_id=$(extract_value "$line" 'top_channel_id')
            local top_channel_name=$(extract_value "$line" 'top_channel_name')
            local top_channel_score=$(extract_value "$line" 'top_channel_score')
            local model=$(extract_value "$line" 'model')
            local duration=$(extract_value "$line" 'duration')
            
            cat << JSON
  {
    "type": "decision",
    "timestamp": "$timestamp",
    "model": "$model",
    "total_channels": $total_channels,
    "selected_channels": $selected_channels,
    "top_channel_id": $top_channel_id,
    "top_channel_name": "$top_channel_name",
    "top_channel_score": $top_channel_score,
    "duration": $duration
  }
JSON
        elif [[ "$line" =~ "Channel load balancing details" ]]; then
            local timestamp=$(extract_value "$line" 'time')
            local channel_id=$(extract_value "$line" 'channel_id')
            local channel_name=$(extract_value "$line" 'channel_name')
            local total_score=$(extract_value "$line" 'total_score')
            local final_rank=$(extract_value "$line" 'final_rank')
            local strategy_breakdown=$(echo "$line" | jq -c '.strategy_breakdown')
            
            cat << JSON
  {
    "type": "channel",
    "timestamp": "$timestamp",
    "channel_id": $channel_id,
    "channel_name": "$channel_name",
    "total_score": $total_score,
    "final_rank": $final_rank,
    "strategy_breakdown": $strategy_breakdown
  }
JSON
        fi
    done
    echo "]"
}

# Format output as CSV
format_as_csv() {
    local input="$1"
    
    echo "type,timestamp,model,channel_id,channel_name,total_score,final_rank,duration"
    
    echo "$input" | while IFS= read -r line; do
        if [[ "$line" =~ "Load balancing decision completed" ]]; then
            local timestamp=$(extract_value "$line" 'time')
            local model=$(extract_value "$line" 'model')
            local top_channel_id=$(extract_value "$line" 'top_channel_id')
            local top_channel_name=$(extract_value "$line" 'top_channel_name')
            local top_channel_score=$(extract_value "$line" 'top_channel_score')
            local duration=$(extract_value "$line" 'duration')
            
            echo "decision,\"$timestamp\",\"$model\",$top_channel_id,\"$top_channel_name\",$top_channel_score,,$duration"
        elif [[ "$line" =~ "Channel load balancing details" ]]; then
            local timestamp=$(extract_value "$line" 'time')
            local channel_id=$(extract_value "$line" 'channel_id')
            local channel_name=$(extract_value "$line" 'channel_name')
            local total_score=$(extract_value "$line" 'total_score')
            local final_rank=$(extract_value "$line" 'final_rank')
            
            echo "channel,\"$timestamp\",,$channel_id,\"$channel_name\",$total_score,$final_rank,"
        fi
    done
}

# Main execution
main() {
    local filtered_logs
    
    # Apply time filter
    filtered_logs=$(filter_by_time "$LOG_CONTENT")
    
    # Apply load balance filters
    filtered_logs=$(filter_load_balance_logs "$filtered_logs")
    
    # Check if any logs found
    if [[ -z "$filtered_logs" ]]; then
        echo -e "${YELLOW}No load balance logs found matching the criteria.${NC}" >&2
        exit 0
    fi
    
    # Show summary if requested
    if [[ "$SHOW_SUMMARY" == true ]]; then
        show_summary "$filtered_logs"
        echo ""
    fi
    
    # Show statistics if requested
    if [[ "$SHOW_STATS" == true ]]; then
        show_stats "$filtered_logs"
        echo ""
    fi
    
    # Show details if requested
    if [[ "$SHOW_DETAILS" == true ]]; then
        show_details "$filtered_logs"
        echo ""
    fi
    
    # Format and output the filtered logs
    if [[ "$SHOW_SUMMARY" == false && "$SHOW_STATS" == false && "$SHOW_DETAILS" == false ]]; then
        case "$OUTPUT_FORMAT" in
            json)
                format_as_json "$filtered_logs"
                ;;
            csv)
                format_as_csv "$filtered_logs"
                ;;
            table)
                format_as_table "$filtered_logs"
                ;;
            *)
                echo -e "${RED}Error: Unknown output format: $OUTPUT_FORMAT${NC}" >&2
                exit 1
                ;;
        esac
    fi
}

# Run main function
main
