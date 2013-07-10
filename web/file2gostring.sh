#!/bin/sh

# this file is copied from doozerd. 

set -e

munge() {
    printf %s "$1" | tr . _ | tr -d -c '[:alnum:]_'
}

quote() {
    sed 's/\\/\\\\/g' | sed 's/"/\\"/g' | sed 's/$/\\n/' | tr -d '\n'
}

pkg_path=$1 ; shift
file=$1     ; shift

pkg=`basename $pkg_path`

printf 'package %s\n' "$pkg"
printf '\n'
printf '// This file was generated from %s.\n' "$file"
printf '\n'
printf 'var '
munge "`basename $file`"
printf ' string = "'
quote
printf '"\n'