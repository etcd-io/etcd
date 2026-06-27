package uv

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}
