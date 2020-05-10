# **IMPORTANT**: This repository is currently in maintenance mode.
For a better autocompletion experience with Go, we suggest you use the Go language server, [gopls](https://github.com/golang/tools/blob/master/gopls/README.md).

If you are looking for a version of `gocode` that works with Go modules, please see [stamblerre/gocode](https://github.com/stamblerre/gocode).

---

[![Build Status](https://travis-ci.org/mdempsky/gocode.svg?branch=master)](https://travis-ci.org/mdempsky/gocode)

## github.com/mdempsky/gocode

This version of gocode is a fork of the [original](https://github.com/nsf/gocode), which is no longer supported. This fork should work for all versions of Go > 1.8. It only works for `$GOPATH` projects. For a version of gocode that works with Modules, please see [github.com/stamblerre/gocode](https://github.com/stamblerre/gocode).

## An autocompletion daemon for the Go programming language

Gocode is a helper tool which is intended to be integrated with your source code editor, like vim, neovim and emacs. It provides several advanced capabilities, which currently includes:

 - Context-sensitive autocompletion

It is called *daemon*, because it uses client/server architecture for caching purposes. In particular, it makes autocompletions very fast. Typical autocompletion time with warm cache is 30ms, which is barely noticeable.

Also watch the [demo screencast](https://nosmileface.dev/images/gocode-demo.swf).

![Gocode in vim](https://nosmileface.dev/images/gocode-screenshot.png)

![Gocode in emacs](https://nosmileface.dev/images/emacs-gocode.png)

### Setup

 1. You should have a correctly installed Go compiler environment and your personal workspace ($GOPATH). If you have no idea what **$GOPATH** is, take a look [here](http://golang.org/doc/code.html). Please make sure that your **$GOPATH/bin** is available in your **$PATH**. This is important, because most editors assume that **gocode** binary is available in one of the directories, specified by your **$PATH** environment variable. Otherwise manually copy the **gocode** binary from **$GOPATH/bin** to a location which is part of your **$PATH** after getting it in step 2.

    Do these steps only if you understand why you need to do them:

    `export GOPATH=$HOME/goprojects`

    `export PATH=$PATH:$GOPATH/bin`

 2. Then you need to install gocode:

    `go get -u github.com/mdempsky/gocode` (-u flag for "update")

    Windows users should consider doing this instead:

    `go get -u -ldflags -H=windowsgui github.com/mdempsky/gocode`

    That way on the Windows OS gocode will be built as a GUI application and doing so solves hanging window issues with some of the editors.
    
    **Note: If you are updating your version of gocode, you will need to run `gocode close` to restart it.**

 3. Next steps are editor specific. See below.

### Vim setup

#### Vim manual installation

Note: As of go 1.5 there is no $GOROOT/misc/vim script. Suggested installation is via [vim-go plugin](https://github.com/fatih/vim-go).

In order to install vim scripts, you need to fulfill the following steps:

 1. Install official Go vim scripts from **$GOROOT/misc/vim**. If you did that already, proceed to the step 2.

 2. Install gocode vim scripts. Usually it's enough to do the following:

    2.1. `vim/update.sh`

    **update.sh** script does the following:

		#!/bin/sh
		mkdir -p "$HOME/.vim/autoload"
		mkdir -p "$HOME/.vim/ftplugin/go"
		cp "${0%/*}/autoload/gocomplete.vim" "$HOME/.vim/autoload"
		cp "${0%/*}/ftplugin/go/gocomplete.vim" "$HOME/.vim/ftplugin/go"

    2.2. Alternatively, you can create symlinks using symlink.sh script in order to avoid running update.sh after every gocode update.

    **symlink.sh** script does the following:

		#!/bin/sh
		cd "${0%/*}"
		ROOTDIR=`pwd`
		mkdir -p "$HOME/.vim/autoload"
		mkdir -p "$HOME/.vim/ftplugin/go"
		ln -s "$ROOTDIR/autoload/gocomplete.vim" "$HOME/.vim/autoload/"
		ln -s "$ROOTDIR/ftplugin/go/gocomplete.vim" "$HOME/.vim/ftplugin/go/"

 3. Make sure vim has filetype plugin enabled. Simply add that to your **.vimrc**:

    `filetype plugin on`

 4. Autocompletion should work now. Use `<C-x><C-o>` for autocompletion (omnifunc autocompletion).

#### Using Vundle in Vim

Add the following line to your **.vimrc**:

`Plugin 'mdempsky/gocode', {'rtp': 'vim/'}`

And then update your packages by running `:PluginInstall`.

#### Using vim-plug in Vim

Add the following line to your **.vimrc**:

`Plug 'mdempsky/gocode', { 'rtp': 'vim', 'do': '~/.vim/plugged/gocode/vim/symlink.sh' }`

And then update your packages by running `:PlugInstall`.

#### Other

Alternatively take a look at the vundle/pathogen friendly repo: https://github.com/Blackrush/vim-gocode.

### Neovim setup
#### Neovim manual installation

 Neovim users should also follow `Vim manual installation`, except that you should goto `gocode/nvim` in step 2, and remember that, the Neovim configuration file is `~/.config/nvim/init.vim`.

#### Using Vundle in Neovim

Add the following line to your **init.vim**:

`Plugin 'mdempsky/gocode', {'rtp': 'nvim/'}`

And then update your packages by running `:PluginInstall`.

#### Using vim-plug in Neovim

Add the following line to your **init.vim**:

`Plug 'mdempsky/gocode', { 'rtp': 'nvim', 'do': '~/.config/nvim/plugged/gocode/nvim/symlink.sh' }`

And then update your packages by running `:PlugInstall`.

### Emacs setup

In order to install emacs script, you need to fulfill the following steps:

 1. Install [auto-complete-mode](http://www.emacswiki.org/emacs/AutoComplete)

 2. Copy **emacs/go-autocomplete.el** file from the gocode source distribution to a directory which is in your 'load-path' in emacs.

 3. Add these lines to your **.emacs**:

 		(require 'go-autocomplete)
		(require 'auto-complete-config)
		(ac-config-default)

Also, there is an alternative plugin for emacs using company-mode. See `emacs-company/README` for installation instructions.

If you're a MacOSX user, you may find that script useful: https://github.com/purcell/exec-path-from-shell. It helps you with setting up the right environment variables as Go and gocode require it. By default it pulls the PATH, but don't forget to add the GOPATH as well, e.g.:

```
(when (memq window-system '(mac ns))
  (exec-path-from-shell-initialize)
  (exec-path-from-shell-copy-env "GOPATH"))
```

### Sublime Text 3 setup

A plugin for Sublime Text 3 is provided in the `subl3` directory of this repository. To install it:

 1. Copy the plugin into your Sublime Text 3 `Packages` directory:

		$ cp -r $GOPATH/src/github.com/mdempsky/gocode/subl3 ~/.config/sublime-text-3/Packages/

 2. Rename the plugin directory from `subl3` to `gocode`:

		$ mv ~/.config/sublime-text-3/Packages/subl3 ~/.config/sublime-text-3/Packages/gocode

 3. Open the Command Pallete (`Ctrl+Shift+P`) and run the `Package Control: List Packages` command. You should see `gocode` listed as an active plugin.

### Debugging

If something went wrong, the first thing you may want to do is manually start the gocode daemon with a debug mode enabled and in a separate terminal window. It will show you all the stack traces, panics if any and additional info about autocompletion requests. Shutdown the daemon if it was already started and run a new one explicitly with a debug mode enabled:

`gocode exit`

`gocode -s -debug`

Please, report bugs, feature suggestions and other rants to the [github issue tracker](http://github.com/mdempsky/gocode/issues) of this project.

### Developing

There is [Guide for IDE/editor plugin developers](docs/IDE_integration.md).

If you have troubles, please, contact me and I will try to do my best answering your questions. You can contact me via <a href="mailto:no.smile.face@gmail.com">email</a>. Or for short question find me on IRC: #go-nuts @ freenode.

### Misc

 - It's a good idea to use the latest git version always. I'm trying to keep it in a working state.
 - Use `go install` (not `go build`) for building a local source tree. The objects in `pkg/` are needed for Gocode to work.
