#!/bin/sh
cd "${0%/*}"
ROOTDIR=`pwd`
mkdir -p "$HOME/.vim/autoload"
mkdir -p "$HOME/.vim/ftplugin/go"
ln -sf "$ROOTDIR/autoload/gocomplete.vim" "$HOME/.vim/autoload/"
ln -sf "$ROOTDIR/ftplugin/go/gocomplete.vim" "$HOME/.vim/ftplugin/go/"
