package normalize

// Normalize run every ruls below, in ordr, and return the result
// The only exported entry point
func Normalize(s string) string {
	panic("TODO: normalizeLetters -> normalizeDigits -> normalizeZWNJ")
}

// maps Arabic form letters to Persian from
// ك (U+0643) -> ک (U+06A9), ي (U+064A) -> ی (U+06CC)
func normalizeLetters(s string) string {
	panic("TODO")
}

// maps English digit into Persian digits
func normalizeDigits(s string) string {
	panic("TODO")
}

func normalizeZWNJ(s string) string {
	panic("TODO")
}
