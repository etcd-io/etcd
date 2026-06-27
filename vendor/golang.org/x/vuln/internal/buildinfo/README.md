This code is a copied and slightly modified subset of go/src/debug/buildinfo.

It contains logic for parsing Go binary files for the purpose of extracting
module dependency and symbol table information.

Logic added by vulncheck is located in files with "additions_" prefix.

Within the originally named files, changed or added logic is annotated with
a comment starting with "Addition:".
