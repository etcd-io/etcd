// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doc

import (
	"flag"
	"io"

	"text/template"
)

// PrintInstallGuide outputs the (HTML) Installation Guide for the Go Doctor.
func PrintInstallGuide(aboutText string, flags *flag.FlagSet, out io.Writer) {
	tmpl := template.Must(template.New("installGuide").Parse(installGuide))
	err := tmpl.Execute(out, struct{ AboutText string }{aboutText})
	if err != nil {
		panic(err)
	}
}

const installGuide = `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en" lang="en">
<head>
  <title>{{.AboutText}} Installation Guide</title>
  <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
  <style>
  html {
    font-family: Arial;
    font-size: 0.688em;
    line-height: 1.364em;
    background-color: white;
    color: black;
  }
  a {
    color: black;
    text-decoration: underline;
  }
  a: hover {
    text-decoration: none;
  }
  tt {
    font-size: 1.2em;
  }
  h1 {
    text-align: center;
    font-size: 2.6em;
    font-weight: bold;
    padding: 20px 0 20px 0;
    background-color: #e0e0e0;
  }
  h2 {
    text-align: left;
    font-size: 2.5em;
    font-weight: bold;
    padding: 10px 0 10px 0;
    background-color: #e0e0e0;
  }
  h3 {
    text-align: left;
    font-size: 1.75em;
    font-weight: bold;
    padding: 5px 0 2px 0;
    border-bottom: 2px dashed #c0c0c0;
  }
  h4 {
    text-align: left;
    font-size: 1.5em;
    font-weight: bold;
    padding: 5px 0 0 0;
  }
  .highlight {
    background-color: yellow;
  }
  .dotted {
    border: 1px dotted;
  }
 
  .clicktoshow {
    display: none;
    font-size: 10px;
    font-weight: normal;
    color: #808080;
  }
  .showable {
    display: block;
  }

  #toc2col .column1 {
    width: 250px;
    padding: 0;
    position: fixed;
    right: 0px;
    top: 0px;
  }
  #toc2col .column2 {
    width: 628px;
    padding: 10px 0 10px 0;
  }

  .box {
    background-color: #c0c0c0;
    width: 210px;
  }
  .box h2 {
    text-align: center;
    font-size: 1.364em;
    line-height: 1em;
    font-weight: bold;
    padding: 3px 0 3px 0;
    margin-top: 40px;
    color:#ffffff;
    background-color: #000000;
  }
  .box ul       { list-style: none; padding: 0; margin: 0; }
  .box ul li    { padding: 5px 0 1px 15px; font-weight: bold;}
  .box ul ul li { padding: 1px 0 1px 30px; font-weight: normal;}

  .man h1 {
    text-align: center;
    font-size: 1.8em;
    line-height: 1em;
    font-weight: bold;
    padding: 3px 0 3px 0;
    margin-top: 5px;
    background-color: #ffffff;
    color: black;
  }
  .man h2 {
    text-align: left;
    font-size: 1.4em;
    line-height: 1em;
    font-weight: bold;
    padding: 3px 0 3px 0;
    margin-top: 20px;
    background-color: #ffffff;
  }

  .vimdoc pre {
    font-size:1.0em;
    margin-left: 20px;
  }
  </style>
  <script language="JavaScript">
    function setDisplay(selectors, value) {
      var divs = document.querySelectorAll(selectors);
      for (var i = 0; i < divs.length; i++) {
        divs[i].style.display = value;
      }
    }

    function show(id) {
      setDisplay('.showable', 'none');
      setDisplay('.clicktoshow', 'block');
      document.getElementById(id).style.display = 'block';
      document.getElementById(id + '-click').style.display = 'none';
    }

    function showAll() {
      setDisplay('.showable', 'block');
      setDisplay('.clicktoshow', 'none');
    }

    function hideAll() {
      setDisplay('.showable', 'none');
      setDisplay('.clicktoshow', 'block');
    }
  </script>
</head>
<body id="toc2col">
    <!-- BEGIN BODY -->
    <div id="middle">
      <div class="container">
        <div class="column1">
          <div class="box">
            <div class="corner_bottom_left">
              <div class="corner_top_right">
                <div class="corner_top_left">
                  <div class="indent">
                    <!-- BEGIN TOC -->
                    <h2>Installation</h2>
                      <ul class="toc2">
                      <li><a onClick="show('install-vim');" href="#install-vim">For vim users</a></li>
                      <li><a onClick="show('install-godoctor');" href="#install-godoctor">For users of other editors</a></li>
                    </ul>
                    <p style="text-align: center; font-size: 10px; color: #808080;">
                      <a href="#" onClick="showAll();">Show All</a> |
                      <a href="#" onClick="hideAll();">Hide All</a>
                    </p>
                    <!-- END TOC -->
                    <br/>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="column2">
          <div class="indent">
            <!-- BEGIN CONTENT -->
<h1>{{.AboutText}} Installation Guide</h1>
<a name="install"></a>
<div id="install">
  <style>
  .question {
    font-family: Arial, helvetica;
    font-weight: bold;
    font-size: 18px;
    text-align: center;
  }
  .answers {
    text-align: center;
  }
  .yes {
    color: #008000;
  }
  .no {
    color: #800000;
  }
  .showable {
    display: none;
  }
  </style>
  <p class="question">Do you use Vim?</p>
  <p class="answers"><a href="#install-vim" onClick="show('install-vim');"><span class="yes">&#x2713;</span>&nbsp;Yes</a>
                     &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
                     <a href="#install-godoctor" onClick="show('install-godoctor');"><span class="no">&#x2717;</span>&nbsp;No</a></p>
</div>
<!-- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -->
<a name="install-vim"></a>
<div id="install-vim" class="showable">
  <h2>Installation for Vim Users</h2>
  <p>The Go Doctor Vim plug-in (godoctor.vim) follows the standard runtime path
  structure, so we recommend using a plug-in manager (like
  <a target="_blank" href="https://github.com/tpope/vim-pathogen">Pathogen</a>)
  to install it.  In addition, <code>filetype plugin indent on</code> must also
  be set (add this to your <code>~/.vimrc</code>).</p>
  <ul>
    <li><a target="_blank" href="https://github.com/tpope/vim-pathogen">Pathogen</a>
      <ul>
      <li><pre>git clone https://github.com/godoctor/godoctor.vim.git \
~/.vim/bundle/godoctor.vim</pre></li>
      </ul></li>
    <li><a target="_blank" href="https://github.com/junegunn/vim-plug">vim-plug</a>
      <ul>
      <li><pre>Plug 'godoctor/godoctor.vim'</pre></li>
      </ul></li>
    <li><a target="_blank" href="https://github.com/Shougo/neobundle.vim">NeoBundle</a>
      <ul>
      <li><pre>NeoBundle 'godoctor/godoctor.vim'</pre></li>
      </ul></li>
    <li><a target="_blank" href="https://github.com/gmarik/vundle">Vundle</a>
      <ul>
      <li><pre>Plugin 'godoctor/godoctor.vim'</pre></li>
      </ul></li>
    <li>Manual
      <ul>
      <li><pre>git clone https://github.com/godoctor/godoctor.vim ~/.vim/godoctor.vim</pre></li>
      <li>Add these lines to ~/.vimrc<br/>
<pre>
if exists("g:did_load_filetypes")
  filetype off
  filetype plugin indent off
endif
set rtp+=~/.vim/godoctor.vim
filetype plugin indent on
syntax on
</pre>
</li>
      </ul></li>
  </ul>
  <p>If the <tt>godoctor</tt> executable is not already installed and in your
  <tt>$PATH</tt>, just type <tt>:GoDoctorInstall</tt>. This will install it to
  <tt>$GOPATH/bin</tt>, which will also need to be in your <tt>$PATH</tt>,
  which assumes you have <tt>$GOPATH</tt> set.  <tt>git</tt> must be installed
  for <tt>:GoDoctorInstall</tt> to work.</p>
  <p>We took express care not to conflict with anything in
  <a href="https://github.com/fatih/vim-go">https://github.com/fatih/vim-go</a>.
  This means (<tt>vim-go</tt> note aside) that it is safe to have
  <tt>vim-go</tt> installed alongside <tt>godoctor.vim</tt>. Notably, vim-go
  has <tt>:GoRename</tt> and <tt>:GoRefactor</tt> where we have
  <tt>:Rename</tt> and <tt>:Refactor</tt> when editing a <tt>.go</tt> file.</p>
</div>
<!-- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -->
<a name="install-godoctor"></a>
<div id="install-godoctor" class="showable">
  <h2>Installation for Users of Other Editors</h2>
  <p>At the moment, the Go Doctor only has a Vim plug-in.  Contributing a
  plug-in for another editor (emacs, Sublime Text, etc.) would be a great way
  to <a href="http://godoctor.com/contrib.html">contribute</a>.</p>
  <p>However, you can still use the <b>Go Doctor command line tool</b>.  To
  install:</p>
  <ul>
    <li><tt>go get github.com/godoctor/godoctor</tt></li>
    <li><tt>go install github.com/godoctor/godoctor</tt></li>
  </ul>
  <p>For examples of how to use the command line tool, see the
  <a href="http://gorefactor.org/doc.html#EXAMPLES"><tt>godoctor</tt> man
  page</a>, or
  <a href="http://gorefactor.org/doc.html#offline">install a local copy of the
  man page</a>.</p>
</div>
            <!-- END CONTENT -->
          </div>
        </div>
      </div>
    </div>
    <!-- END BODY -->
</body>
</html>
`
