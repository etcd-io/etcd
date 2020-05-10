package testdata

func SameName() (int, error) {
	return 0, nil
}

func sameName() (int, error) {
	return 0, nil
}

func (t *SameTypeName) SameName() (int, error) {
	return 0, nil
}

func (t *SameTypeName) sameName() (int, error) {
	return 0, nil
}

func (t *sameTypeName) SameName() (int, error) {
	return 0, nil
}

func (t *sameTypeName) sameName() (int, error) {
	return 0, nil
}
