#!/bin/sh
mkdir -p "$HOME/.config/nvim/autoload"
mkdir -p "$HOME/.config/nvim/ftplugin/go"
cp "${0%/*}/autoload/gocomplete.vim" "$HOME/.config/nvim/autoload"
cp "${0%/*}/ftplugin/go/gocomplete.vim" "$HOME/.config/nvim/ftplugin/go"
