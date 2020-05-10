;;; go-keyify.el --- keyify integration for Emacs

;; Copyright 2016 Dominik Honnef. All rights reserved.
;; Use of this source code is governed by a BSD-style
;; license that can be found in the LICENSE file.

;; Author: Dominik Honnef
;; Version: 1.0.0
;; Keywords: languages go
;; URL: https://github.com/dominikh/go-keyify
;;
;; This file is not part of GNU Emacs.

;;; Code:

(require 'json)

;;;###autoload
(defun go-keyify ()
  "Turn an unkeyed struct literal into a keyed one.

Call with point on or in a struct literal."
  (interactive)
  (let* ((name (buffer-file-name))
         (point (point))
         (bpoint (1- (position-bytes point)))
         (out (get-buffer-create "*go-keyify-output")))
    (with-current-buffer out
      (setq buffer-read-only nil)
      (erase-buffer))
    (with-current-buffer (get-buffer-create "*go-keyify-input*")
      (setq buffer-read-only nil)
      (erase-buffer)
      (go--insert-modified-files)
      (call-process-region (point-min) (point-max) "keyify" t out nil
                           "-modified"
                           "-json"
                           (format "%s:#%d" name bpoint)))
    (let ((res (with-current-buffer out
                 (goto-char (point-min))
                 (json-read))))
      (delete-region
       (1+ (cdr (assoc 'start res)))
       (1+ (cdr (assoc 'end res))))
      (insert (cdr (assoc 'replacement res)))
      (indent-region (1+ (cdr (assoc 'start res))) (point))
      (goto-char point))))

(provide 'go-keyify)

;;; go-keyify.el ends here
