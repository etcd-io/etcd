package pkg

import "regexp"

func fn() {
	var r *regexp.Regexp
	_ = r.FindAll(nil, 0) //want `calling a FindAll method with n == 0 will return no results`
}
