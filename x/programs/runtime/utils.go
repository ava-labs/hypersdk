package runtime

var None = []byte{0}

func Ok(data []byte) []byte {
	return append([]byte{1}, data...)
}

func Err(data []byte) []byte {
	return append([]byte{0}, data...)
}

func Some(data []byte) []byte {
	return append([]byte{1}, data...)
}
