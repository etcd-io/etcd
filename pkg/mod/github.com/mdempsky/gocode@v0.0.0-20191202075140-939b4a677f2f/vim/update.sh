#!/bin/sh
mkdir -p "$HOME/.vim/autoload"
mkdir -p "$HOME/.vim/ftplugin/go"
cp "${0%/*}/autoload/gocomplete.vim" "$HOME/.vim/autoload"
cp "${0%/*}/ftplugin/go/gocomplete.vim" "$HOME/.vim/ftplugin/go"
