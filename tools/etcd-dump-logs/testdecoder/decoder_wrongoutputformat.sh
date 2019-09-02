#!/bin/bash
while read line
do
  echo "$line" | tr 1234567890 abcdefghij
done < "${1:-/dev/stdin}"
