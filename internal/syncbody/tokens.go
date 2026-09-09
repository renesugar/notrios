package syncbody

import "unicode"

// SplitLines returns physical lines with their terminators attached, so that
// concatenating the result reproduces the input byte for byte. A merge that
// cannot round-trip its own tokenization would silently rewrite line endings
// in notes it merely relayed.
func SplitLines(text string) []string {
	if text == "" {
		return nil
	}
	tokens := make([]string, 0, 16)
	start := 0
	for index := 0; index < len(text); index++ {
		if text[index] != '\n' {
			continue
		}
		tokens = append(tokens, text[start:index+1])
		start = index + 1
	}
	if start < len(text) {
		tokens = append(tokens, text[start:])
	}
	return tokens
}

// SplitWords tokenizes a conflict region into Unicode word, whitespace, and
// punctuation runs. Words are maximal runs of letters, digits, marks, and
// connector punctuation, so "don't" is three tokens and "café" is one.
// Whitespace runs stay whole. Every other rune is its own token, which keeps a
// run of symbols from merging into an opaque blob that can only conflict.
//
// Concatenating the result reproduces the input exactly.
func SplitWords(text string) []string {
	if text == "" {
		return nil
	}
	tokens := make([]string, 0, 32)
	runes := []rune(text)
	for index := 0; index < len(runes); {
		start := index
		switch {
		case isWordRune(runes[index]):
			for index < len(runes) && isWordRune(runes[index]) {
				index++
			}
		case unicode.IsSpace(runes[index]):
			for index < len(runes) && unicode.IsSpace(runes[index]) {
				index++
			}
		default:
			index++
		}
		tokens = append(tokens, string(runes[start:index]))
	}
	return tokens
}

func isWordRune(value rune) bool {
	return unicode.IsLetter(value) || unicode.IsDigit(value) || unicode.IsMark(value) || value == '_'
}

func joinTokens(tokens []string) string {
	total := 0
	for _, token := range tokens {
		total += len(token)
	}
	joined := make([]byte, 0, total)
	for _, token := range tokens {
		joined = append(joined, token...)
	}
	return string(joined)
}

func equalTokens(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
