#!/bin/bash
while read line
do
  LEN=$(echo ${#line})
  if [ $LEN -ge 20 ]; then
    echo "OK|$line" | tr 1234567890 abcdefghij
  else
    echo "ERROR|$line" | tr 1234567890 abcdefghij
  fi
done < "${1:-/dev/stdin}"
