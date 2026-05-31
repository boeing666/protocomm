package protocomm

func Fnv1a32(s string) uint32 {
	hash := uint32(0x811c9dc5)
	for i := 0; i < len(s); i++ {
		hash ^= uint32(s[i])
		hash *= 0x01000193
	}
	return hash
}

func MethodID(pkg, service, method string) uint32 {
	if pkg == "" {
		return Fnv1a32("/" + service + "/" + method)
	}
	return Fnv1a32("/" + pkg + "." + service + "/" + method)
}
