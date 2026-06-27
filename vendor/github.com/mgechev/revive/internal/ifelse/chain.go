package ifelse

// Chain contains information about an if-else chain.
type Chain struct {
	If                   Branch     // what happens at the end of the "if" block
	HasElse              bool       // is there an "else" block?
	Else                 Branch     // what happens at the end of the "else" block
	HasInitializer       bool       // is there an "if"-initializer somewhere in the chain?
	HasPriorNonDeviating bool       // is there a prior "if" block that does NOT deviate control flow?
	AtBlockEnd           bool       // whether the chain is placed at the end of the surrounding block
	BlockEndKind         BranchKind // control flow at end of surrounding block (e.g. "return" for function body)
}
