# Contributing

The files `words.go`, `works_uk.go`, and `works_us.go` must never be edited by hand.

## Adding a word

Misspell is neither a complete spell-checking program nor a grammar checker.
It is a tool to correct commonly misspelled English words.

The list of words must contain only common mistakes.

Before adding a word, you should have information about the misspelling frequency.

- [ ] more than 15k inside GitHub (this limit is arbitrary and can involve)
- [ ] don't exist inside the [Wiktionary](https://en.wiktionary.org/wiki/) (as a modern form) 
- [ ] don't exist inside the [Cambridge Dictionary](https://dictionary.cambridge.org) (as a modern form)
- [ ] don't exist inside the [Oxford Dictionary](https://www.oed.com/search/dictionary/) (as a modern form)

If all criteria are met, a word can be added to the list of misspellings.

The word should be added to one of the following files.

- `internal/gen/sources/main.json`: common words.
- `internal/gen/sources/uk.json`: UK only words.
- `internal/gen/sources/us.json`: US only words.

The target `make generate` will generate the Go files.

The PR description must provide all the information (links) about the misspelling frequency.
