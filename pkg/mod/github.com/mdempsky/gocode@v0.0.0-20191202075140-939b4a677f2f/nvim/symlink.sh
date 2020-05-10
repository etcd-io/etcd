#!/bin/sh
cd "${0%/*}"
ROOTDIR=`pwd`
mkdir -p "$HOME/.config/nvim/autoload"
mkdir -p "$HOME/.config/nvim/ftplugin/go"
ln -s "$ROOTDIR/autoload/gocomplete.vim" "$HOME/.config/nvim/autoload/"
ln -s "$ROOTDIR/ftplugin/go/gocomplete.vim" "$HOME/.config/nvim/ftplugin/go/"
