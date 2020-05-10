# Company-go
Company-go is an alternative emacs plugin for autocompletion. It uses [company-mode](http://company-mode.github.io).
Completion will start automatically whenever the current symbol is preceded by a `.`, or after you type `company-minimum-prefix-length` letters.

## Setup
Install `company` and `company-go`.

Add the following to your emacs-config:

```lisp
(require 'company)                                   ; load company mode
(require 'company-go)                                ; load company mode go backend
```

## Possible improvements

```lisp
(setq company-tooltip-limit 20)                      ; bigger popup window
(setq company-idle-delay .3)                         ; decrease delay before autocompletion popup shows
(setq company-echo-delay 0)                          ; remove annoying blinking
(setq company-begin-commands '(self-insert-command)) ; start autocompletion only after typing
```

### Only use company-mode with company-go in go-mode
By default company-mode loads every backend it has. If you want to only have company-mode enabled in go-mode add the following to your emacs-config:

```lisp
(add-hook 'go-mode-hook (lambda ()
                          (set (make-local-variable 'company-backends) '(company-go))
                          (company-mode)))
```

### Color customization

```lisp
(custom-set-faces
 '(company-preview
   ((t (:foreground "darkgray" :underline t))))
 '(company-preview-common
   ((t (:inherit company-preview))))
 '(company-tooltip
   ((t (:background "lightgray" :foreground "black"))))
 '(company-tooltip-selection
   ((t (:background "steelblue" :foreground "white"))))
 '(company-tooltip-common
   ((((type x)) (:inherit company-tooltip :weight bold))
    (t (:inherit company-tooltip))))
 '(company-tooltip-common-selection
   ((((type x)) (:inherit company-tooltip-selection :weight bold))
    (t (:inherit company-tooltip-selection)))))
```
