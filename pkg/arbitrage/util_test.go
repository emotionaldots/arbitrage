package arbitrage

import (
	"fmt"
	"testing"
)

type roundingTest struct {
	Exact   int64
	Rounded float64
}

func TestRoundBytes(t *testing.T) {
	tests := []roundingTest{
		{25243636, 24.07 * 1024 * 1024},
		{23270403, 22.19 * 1024 * 1024},
		{28923494, 27.58 * 1024 * 1024},
		{23995641, 22.88 * 1024 * 1024},
		{28916664, 27.58 * 1024 * 1024},
		{27671836, 26.39 * 1024 * 1024},
		{24410592, 23.28 * 1024 * 1024},
		{18789710, 17.92 * 1024 * 1024},
		{34107090, 32.53 * 1024 * 1024},
		{19908130, 18.99 * 1024 * 1024},
		{27516173, 26.24 * 1024 * 1024},
		{45641, 44.57 * 1024},
		{2515, 2.46 * 1024},
		{9183, 8.97 * 1024},
	}

	for _, test := range tests {
		fmt.Printf("%d\n%d\n%d\n%d\n\n", test.Exact, int64(test.Rounded), RoundBytes(test.Exact), RoundBytes(int64(test.Rounded)))
	}
}
