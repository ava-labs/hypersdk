package actions

// https://github.com/Uniswap/v2-core/blob/ee547b17853e71ed4e0101ccfd52e70d5acded58/contracts/libraries/Math.sol#L10
func sqrt(y uint64) uint64 {
	if y > 3 {
		z := y
		x := (y / 2) + 1
		for x < z {
			z = x
			x = (y / x + x) / 2
		}
		return z
	} else if y != 0 {
		return 1
	}
	return 0
}

func min (x uint64, y uint64) uint64 {
	if x < y {
		return x
	} else {
		return y
	}
}