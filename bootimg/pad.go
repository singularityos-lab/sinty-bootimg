package bootimg

func align(n, to int) int { return (n + to - 1) / to * to }

func padTo(b []byte, to int) []byte {
	return append(b, make([]byte, align(len(b), to)-len(b))...)
}

func putString(dst []byte, s string) error {
	if len(s) >= len(dst) {
		return errTooLong(len(dst))
	}
	copy(dst, s)
	return nil
}
