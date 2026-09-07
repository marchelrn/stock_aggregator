package utils

func SafeCompareString(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	lenA := len(a)
	var result byte = 0
	for i := range lenA {
		result |= a[i] ^ b[i]
	}

	return result == 0
}
