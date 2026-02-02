#!/usr/bin/env bash
# Scale ASCII art to fit terminal width (and optionally height).
# Usage: ./scripts/ascii-fit.sh [file] [width]
#   file   - path to ASCII art file (default: asciit+ in repo root)
#   width  - target columns (default: terminal width from tput cols)
#
# To fit both width and height, pipe to less: ./scripts/ascii-fit.sh asciit+ | less

set -e
FILE="${1:-$(git rev-parse --show-toplevel 2>/dev/null)/asciit+}"
WIDTH="${2:-$(tput cols 2>/dev/null || echo 80)}"

if [[ ! -f "$FILE" ]]; then
  echo "Usage: $0 <ascii-art-file> [width]" >&2
  echo "  File not found: $FILE" >&2
  exit 1
fi

# Scale each line to WIDTH by sampling (integer math, no bc)
while IFS= read -r line || [[ -n "$line" ]]; do
  len=${#line}
  if (( len <= WIDTH )); then
    printf '%s\n' "$line"
  else
    out=""
    for (( i = 0; i < WIDTH; i++ )); do
      idx=$(( (i * len) / WIDTH ))
      out="${out}${line:idx:1}"
    done
    printf '%s\n' "$out"
  fi
done < "$FILE"
