package ginkgohandler

const ( // container names
	describe  = "Describe"
	pdescribe = "PDescribe"
	xdescribe = "XDescribe"
	fdescribe = "FDescribe"

	when  = "When"
	pwhen = "PWhen"
	xwhen = "XWhen"
	fwhen = "FWhen"

	contextContainer = "Context"
	pcontext         = "PContext"
	xcontext         = "XContext"
	fcontext         = "FContext"

	it  = "It"
	pit = "PIt"
	xit = "XIt"
	fit = "FIt"

	describeTable  = "DescribeTable"
	pdescribeTable = "PDescribeTable"
	xdescribeTable = "XDescribeTable"
	fdescribeTable = "FDescribeTable"

	entry  = "Entry"
	pentry = "PEntry"
	xentry = "XEntry"
	fentry = "FEntry"
)

func isFocusContainer(name string) bool {
	switch name {
	case fdescribe, fcontext, fwhen, fit, fdescribeTable, fentry:
		return true
	}
	return false
}

func isContainer(name string) bool {
	switch name {
	case it, when, contextContainer, describe, describeTable, entry,
		pit, pwhen, pcontext, pdescribe, pdescribeTable, pentry,
		xit, xwhen, xcontext, xdescribe, xdescribeTable, xentry:
		return true
	}
	return isFocusContainer(name)
}

func isWrapContainer(name string) bool {
	switch name {
	case when, contextContainer, describe,
		fwhen, fcontext, fdescribe,
		pwhen, pcontext, pdescribe,
		xwhen, xcontext, xdescribe:
		return true
	}

	return false
}
