package ttlcache

type tsHeap[K comparable, V any] []*item[K, V]

func (t tsHeap[K, V]) Len() int {
	return len(t)
}

func (t tsHeap[K, V]) Less(i, j int) bool {
	return t[i].expireAt < t[j].expireAt
}

func (t tsHeap[K, V]) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
	t[i].expireIndex = i
	t[j].expireIndex = j
}

func (t *tsHeap[K, V]) Push(it any) {
	v := it.(*item[K, V])

	v.expireIndex = len(*t)
	*t = append(*t, v)
}

func (t *tsHeap[K, V]) Pop() any {
	arr := *t
	lastInd := len(arr) - 1

	v := arr[lastInd]
	arr[lastInd] = nil
	arr = arr[:lastInd]
	*t = arr

	v.expireIndex = -1

	return v
}
