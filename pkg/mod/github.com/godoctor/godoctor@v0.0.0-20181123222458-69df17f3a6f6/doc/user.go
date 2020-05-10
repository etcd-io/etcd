// Copyright 2015-2017 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doc

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"text/template"
)

type UserGuideContent struct {
	docContent
	ManPageHTML string
	VimdocHTML  string
}

// PrintUserGuide outputs the User's Guide for the Go Doctor (in HTML).
//
// Both the godoctor man page and the Vim plugin reference are generated and
// included in the User's Guide.  The man page content is piped through groff
// to convert it to HTML.
func PrintUserGuide(aboutText string, flags *flag.FlagSet, out io.Writer) {
	PrintUserGuideAsGiven(aboutText, flags, &UserGuideContent{}, out)
}

// PrintUserGuideAsGiven outputs the User's Guide for the Go Doctor (in HTML).
// However, if the content's ManPageHTML and/or VimdocHTML is nonempty, the
// given content is used rather than generating the content.  This is used by
// the online documentation, which cannot execute groff to convert the man page
// to HTML (due to an App Engine restriction), and which uses a Vim-colored
// version of the Vim plugin documentation.
func PrintUserGuideAsGiven(aboutText string, flags *flag.FlagSet, ctnt *UserGuideContent, out io.Writer) {
	ctnt.docContent = prepare(aboutText, flags)
	if ctnt.ManPageHTML == "" {
		ctnt.ManPageHTML = extractBetween(convertManPage(aboutText, flags),
			"<body>", "</body>")
	}
	if ctnt.VimdocHTML == "" {
		ctnt.VimdocHTML = fmt.Sprintf(
			"<pre>\n%s\n</pre>",
			printVimdoc(aboutText, flags))
	}

	tmpl := template.Must(template.New("userGuide").Parse(userGuide))
	err := tmpl.Execute(out, ctnt)
	if err != nil {
		panic(err)
	}
}

func convertManPage(aboutText string, flags *flag.FlagSet) string {
	cmd := exec.Command("groff", "-t", "-mandoc", "-Thtml")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Sprintf("[ERROR] %s", err.Error())
	}

	go func() {
		defer func() {
			recover()
		}()
		PrintManPage(aboutText, flags, stdin)
		err = stdin.Close()
	}()

	result, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("[ERROR] %s", err.Error())
	}
	return string(result)
}

func extractBetween(s, from, to string) string {
	i := strings.Index(s, from)
	if i < 0 {
		return ""
	}

	j := strings.LastIndex(s, to)
	if j < 0 {
		return ""
	}

	return s[i+len(from) : j]
}

func printVimdoc(aboutText string, flags *flag.FlagSet) string {
	defer func() {
		recover()
	}()
	var b bytes.Buffer
	PrintVimdoc(aboutText, flags, &b)
	return b.String()
}

const userGuide = `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en" lang="en">
<head>
  <title>{{.AboutText}} User's Guide</title>
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
    display: show;
    //float: right;
    font-size: 10px;
    font-weight: normal;
    color: #808080;
  }
  .showable {
    display: none;
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
                    <h2>Getting Started</h2>
                    <ul class="toc2">
                      <li><a onClick="show('starting');" href="#starting">Getting Started</a></li>
                      <li><a onClick="show('documentation');" href="#documentation">Online Documentation</a></li>
                      <li><a onClick="show('offline');" href="#offline">Installing Local Documentation</a></li>
                    </ul>
                    <h2>Refactorings</h2>
                    <ul class="toc2">
                      {{range .Refactorings}}
                      <li><a onClick="show('refactoring-{{.Key}}');" href="#refactoring-{{.Key}}">{{.Description.Name}}</a></li>
                      {{end}}
                    </ul>
                    <h2>References</h2>
                    <ul class="toc2">
                      <li><a onClick="show('godoctor-man');" href="#godoctor-man">Man Page (<tt>godoctor.1</tt>)</a></li>
                      <li><a onClick="show('godoctor-vim');" href="#godoctor-vim">Vim Plug-in Reference</a></li>
                      <li><a onClick="show('license');" href="#license">License</a></li>
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
<h1>{{.AboutText}} User's Guide</h1>
<a name="starting"></a>
<h3>Getting Started</h3>
<div id="starting-click" class="clicktoshow">
  <a href="#starting" onClick="show('starting');">Show&nbsp;&raquo;</a>
</div>
<div id="starting" class="showable">
  <p>If this is your first time using the Go Doctor, see the
  <a href="http://gorefactor.org/starting.html">Getting Started</a> page on the
  Go Doctor Web site.  It contains videos (and basic instructions) showing some
  typical uses of the Go Doctor using the Vim plug-in.</p>
  <p>You will probably want to run the Go Doctor from Vim, since it allows you
  to rename variables, extract functions, and perform other refactorings with
  just a few keystrokes.  Detailed information about the Vim plug-in is
  available in the <a onClick="show('godoctor-vim');" href="#godoctor-vim">Vim
  Plug-in Reference</a>.  If you are inclined to use the command-line
  <tt>godoctor</tt> tool directly, the <a onClick="show('godoctor-man');"
  href="#godoctor-man"><tt>godoctor</tt> man page</a> contains several examples
  illustrating typical invocations.</p>
  <p>Later parts of this user manual contain detailed information on each of
  the refactorings supported by the Go Doctor.</p>
</div>
<a name="documentation"></a>
<h3>Online Documentation</h3>
<div id="documentation-click" class="clicktoshow">
  <a href="#documentation" onClick="show('documentation');">Show&nbsp;&raquo;</a>
</div>
<div id="documentation" class="showable">
  <p>The latest documentation for the Go Doctor (including this user manual,
  the <tt>godoctor</tt> man page, and the Vim Plug-in Reference) is always
  available online at <a target="_blank"
  href="http://gorefactor.org">http://gorefactor.org</a>.  If you want to
  install a local copy of the documentation, see below.</p>
</div>
<a name="offline"></a>
<h3>Installing Local Documentation</h3>
<div id="offline-click" class="clicktoshow">
  <a href="#offline" onClick="show('offline');">Show&nbsp;&raquo;</a>
</div>
<div id="offline" class="showable">
  <p>If you do not want to use the online documentation, you can install the Go
  Doctor documentation locally.  The <tt>godoctor</tt> tool generates its own
  documentation.  It can generate a local copy of the Installation Guide and
  User Guide on the Go Doctor Web site; it can generate a Unix man page for the
  <tt>godoctor</tt> tool; it can also generate more complete documentation for
  the Vim plug-in (by default, the Vim plug-in includes stub documentation
  pointing to the Go Doctor Web site).</p>
  <ul>
    <li><b>To generate a local copy of the Installation Guide:</b><br/><br/>
      &nbsp;&nbsp;&nbsp;&nbsp;
      <tt>godoctor -doc install &gt;install.html</tt></li>
    <li><b>To generate a local copy of the User's Guide:</b><br/><br/>
      &nbsp;&nbsp;&nbsp;&nbsp;
      <tt>godoctor -doc user &gt;user.html</tt></li>
    <li><b>To generate a local copy of the godoctor man page,</b><br/><br/>
      use the <tt>godoctor</tt> tool to generate the man page, then save it as
      <i>godoctor.1</i> in a directory on your MANPATH.  For example, if you
      installed <tt>godoctor</tt> into /usr/local/bin, you will probably want
      to install the man page into /usr/local/share/man:<br/><br/>
      <pre>sudo mkdir -p /usr/local/share/man/man1</pre><br/>
      <pre>sudo sh -c 'godoctor -doc man &gt;/usr/local/share/man/man1/godoctor.1'</pre><br/></li>
    <li><b>To install complete documentation for the Vim plug-in:</b>
    <ol class="enum">
      <li>Use the <tt>godoctor</tt> tool to generate the Vim documentation, and
        overwrite the <b>godoctor-vim.txt</b> file in the Vim plug-in's
        <i>doc</i> directory.  For example:<br/><br/>
        <pre>godoctor -doc vim &gt;~/.vim/godoctor.vim/doc/godoctor-vim.txt</pre></li>
      <li>In Vim, run<br/><br/>
        <pre>:helptags ~/.vim/godoctor.vim/doc</pre><br/>
        to generate help tags from the plug-in documentation.</li>
      <li>To view the Vim plug-in documentation, execute<br/><br/>
        <tt>:help godoctor</tt></li>
    </ol>
  </ul>
</div>
<a name="refactorings"></a>
<h2>Refactorings</h2>
<div id="refactorings"></div>
{{range .Refactorings}}
<a name="refactoring-{{.Key}}"></a>
<h3>Refactoring: {{.Description.Name}}</h3>
<div id="refactoring-{{.Key}}-click" class="clicktoshow">
  <a href="#refactoring-{{.Key}}" onClick="show('refactoring-{{.Key}}');">Show&nbsp;&raquo;</a>
</div>
<div id="refactoring-{{.Key}}" class="showable">
  {{.Description.HTMLDoc}}
</div>
{{end}}
<div id="references">
  <a name="references"></a>
  <h2>References</h2>
</div>
<a name="godoctor-man"></a>
<h3><tt>godoctor</tt> Man Page</h3>
<div id="godoctor-man-click" class="clicktoshow">
  <a href="#godoctor-man" onClick="show('godoctor-man');">Show&nbsp;&raquo;</a>
</div>
<div id="godoctor-man" class="showable">
  <div class="man">
    {{.ManPageHTML}}
  </div>
</div>
<a name="godoctor-vim"></a>
<h3>Vim Plugin Reference</h3>
<div id="godoctor-vim-click" class="clicktoshow">
  <a href="#godoctor-vim" onClick="show('godoctor-vim');">Show&nbsp;&raquo;</a>
</div>
<div id="godoctor-vim" class="showable">
  <div class="vimdoc">
  {{.VimdocHTML}}
  </div>
</div>
<a name="license"></a>
<h3>License</h3>
<div id="license-click" class="clicktoshow">
  <a href="#license" onClick="show('license');">Show&nbsp;&raquo;</a>
</div>
<div id="license" class="showable">
  <p>Copyright &copy; Auburn University and others.  All rights reserved.</p>
  <p>Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:</p>
  <p>1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.</p>
  <p>2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.</p>
  <p>3. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.</p>
  <p>THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.</p>
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
