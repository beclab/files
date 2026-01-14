#!/bin/bash

if [[ -n "$RESERVED_SPACE_PERCENT" ]]; then
  PERCENT=$(echo "$RESERVED_SPACE_PERCENT * 100" | bc | sed 's/\.00$//')
else
  PERCENT=100
fi

get_disk_info() {
  local path="$1"
  IFS=" " read -r TOTAL FREE < <(
    df -P -k "$path" 2>/dev/null | awk 'NR==2 {print $2, $4}'
  )
}

get_disk_info "$1"
ORIG_TOTAL=$TOTAL
ORIG_FREE=$FREE

if [[ $ORIG_TOTAL -ge 1000000000000 ]]; then
  get_disk_info /
  TOTAL=$TOTAL
  FREE=$FREE
else
  TOTAL=$ORIG_TOTAL
  FREE=$ORIG_FREE
fi

RESERVED=$((TOTAL * PERCENT / 10000))
ADJUSTED_FREE=$((FREE - RESERVED))

echo "$TOTAL $ADJUSTED_FREE"