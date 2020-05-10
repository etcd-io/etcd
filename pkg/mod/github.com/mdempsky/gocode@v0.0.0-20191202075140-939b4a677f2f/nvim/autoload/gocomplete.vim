if exists('g:loaded_gocode')
	finish
endif
let g:loaded_gocode = 1

fu! s:gocodeCurrentBuffer()
	let buf = getline(1, '$')
	if &encoding != 'utf-8'
		let buf = map(buf, 'iconv(v:val, &encoding, "utf-8")')
	endif
	if &l:fileformat == 'dos'
		" XXX: line2byte() depend on 'fileformat' option.
		" so if fileformat is 'dos', 'buf' must include '\r'.
		let buf = map(buf, 'v:val."\r"')
	endif
	let file = tempname()
	call writefile(buf, file)
	return file
endf

let s:vim_system = get(g:, 'gocomplete#system_function', 'system')

fu! s:system(str, ...)
	return call(s:vim_system, [a:str] + a:000)
endf

fu! s:gocodeShellescape(arg)
	try
		let ssl_save = &shellslash
		set noshellslash
		return shellescape(a:arg)
	finally
		let &shellslash = ssl_save
	endtry
endf

fu! s:gocodeCommand(cmd, preargs, args)
	for i in range(0, len(a:args) - 1)
		let a:args[i] = s:gocodeShellescape(a:args[i])
	endfor
	for i in range(0, len(a:preargs) - 1)
		let a:preargs[i] = s:gocodeShellescape(a:preargs[i])
	endfor
	let result = s:system(printf('gocode %s %s %s', join(a:preargs), a:cmd, join(a:args)))
	if v:shell_error != 0
		return "[\"0\", []]"
	else
		if &encoding != 'utf-8'
			let result = iconv(result, 'utf-8', &encoding)
		endif
		return result
	endif
endf

fu! s:gocodeCurrentBufferOpt(filename)
	return '-in=' . a:filename
endf

fu! s:gocodeCursor()
	if &encoding != 'utf-8'
		let c = col('.')
		let buf = line('.') == 1 ? "" : (join(getline(1, line('.')-1), "\n") . "\n")
		let buf .= c == 1 ? "" : getline('.')[:c-2]
		return printf('%d', len(iconv(buf, &encoding, "utf-8")))
	endif
	return printf('%d', line2byte(line('.')) + (col('.')-2))
endf

fu! s:gocodeAutocomplete()
	let filename = s:gocodeCurrentBuffer()
	let result = s:gocodeCommand('autocomplete',
				   \ [s:gocodeCurrentBufferOpt(filename), '-f=vim'],
				   \ [expand('%:p'), s:gocodeCursor()])
	call delete(filename)
	return result
endf

fu! gocomplete#Complete(findstart, base)
	"findstart = 1 when we need to get the text length
	if a:findstart == 1
		execute "silent let g:gocomplete_completions = " . s:gocodeAutocomplete()
		return col('.') - g:gocomplete_completions[0] - 1
	"findstart = 0 when we need to return the list of completions
	else
		return g:gocomplete_completions[1]
	endif
endf
