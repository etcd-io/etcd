#!/bin/sh
mkdir -p "$HOME/.vim/bundle/gocode/autoload"
mkdir -p "$HOME/.vim/bundle/gocode/ftplugin/go"
cp "${0%/*}/autoload/gocomplete.vim" "$HOME/.vim/bundle/gocode/autoload"
cp "${0%/*}/ftplugin/go/gocomplete.vim" "$HOME/.vim/bundle/gocode/ftplugin/go"
