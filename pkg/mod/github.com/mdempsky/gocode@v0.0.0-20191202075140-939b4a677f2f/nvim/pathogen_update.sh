#!/bin/sh
if [ -z $XDG_CONFIG_HOME ]; then
	XDG_CONFIG_HOME="$HOME/.config"
fi
mkdir -p "$XDG_CONFIG_HOME/nvim/bundle/gocode/autoload"
mkdir -p "$XDG_CONFIG_HOME/nvim/bundle/gocode/ftplugin/go"
cp "${0%/*}/autoload/gocomplete.vim" "$XDG_CONFIG_HOME/nvim/bundle/gocode/autoload"
cp "${0%/*}/ftplugin/go/gocomplete.vim" "$XDG_CONFIG_HOME/nvim/bundle/gocode/ftplugin/go"
