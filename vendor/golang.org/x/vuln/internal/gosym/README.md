This code is a copied and slightly modified version of go/src/debug/gosym.

The original code contains logic for accessing symbol tables and line numbers
in Go binaries. The only reason why this is copied is to support inlining.

Code added by vulncheck is located in files with "additions_" prefix and it
contains logic for accessing inlining information.

Within the originally named files, deleted or added logic is annotated with
a comment starting with "Addition:". The modified logic allows the inlining
code in "additions_*" files to access the necessary information.
