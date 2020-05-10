# Sublime Text 3 plugin

If you want full Go experience in sublime, use [GoSublime](https://github.com/DisposaBoy/GoSublime).

This plugin is written by me (nsf) as a result of frustration with GoSublime. I wanted something simpler and plugin has to use system version of the gocode. As you may know, GoSublime uses a fork of gocode, which is integrated into its own daemon called MarGo (as far as I know).

The plugin does:

1. Basic gocode autocompletion. I did my best presenting the results to the user. Maybe it's possible to improve that part a bit, not quite sure yet. For functions I use "{**class**} {**name**}\t{**returns**}", for everything else "{**class**} {**name**}\t{**type**}". In addition to that functions are properly augmented with argument snippets.

2. Gofmt on save. Here I use difflib to calculate differences, which results in line by line changes. GoSublime uses [google-diff-match-patch](https://code.google.com/archive/p/google-diff-match-patch/) library, which calculates changes on finer granularity, maybe I should switch to that lib as well.

I don't have any big plans on this plugin. The reason why I moved to sublime in the first place is, again, frustration, but with Atom (too slow). We'll see how it goes.

Oh yes, this plugin uses GoSublime syntax files for Go. Well, they use a fork of gocode, I'll use the fork of their syntax files, fair deal.
