package store

type NotFoundError string

func (e NotFoundError) Error() string {
	return string(e)
}

type NotFile string

func (e NotFile) Error() string {
	return string(e)
}

type TestFail string

func (e TestFail) Error() string {
	return string(e)
}

type Keyword string

func (e Keyword) Error() string {
	return string(e)
}
