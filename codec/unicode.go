package codec

type UnicodeBytes []byte

func (b UnicodeBytes) MarshalText() ([]byte, error) {
	return []byte(b), nil
}

func (b *UnicodeBytes) UnmarshalText(text []byte) error {
	*b = text
	return nil
}
