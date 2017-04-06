package arbitrage

func RoundBytes(b int64) int64 {
	f := float64(b)
	exp := 0
	for exp = 0; f >= 1024; exp++ {
		f /= 1024
	}
	rounded := int64(f*100 + 0.5)
	expanded := float64(rounded) / 100
	for exp > 0 {
		exp--
		expanded *= 1024
	}
	return int64(expanded)
}
