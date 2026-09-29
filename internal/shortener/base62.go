package shortener

const base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func EncodeBase62(id int64) string {
	if id == 0 {
		return string(base62Alphabet[0])
	}
	var buf []byte
	for id > 0 {
		remainder := id % 62
		buf = append([]byte{base62Alphabet[remainder]}, buf...)
		id /= 62
	}
	return string(buf)
}
