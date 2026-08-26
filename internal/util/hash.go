package util

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"strings"
	"unsafe"
)

const (
	// LineHashLen is the number of lowercase letters in a per-line hash.
	LineHashLen = 3
	lineHashMod = 26 * 26 * 26 // 17576

	// FileHashLen is the number of uppercase hex digits in a whole-file tag.
	FileHashLen = 4
)

// removeWhitespace removes all whitespace characters from s (optimized, no regex).
func removeWhitespace(s string) string {
	hasWhitespace := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || (c >= 0x0B && c <= 0x0D) {
			hasWhitespace = true
			break
		}
	}
	if !hasWhitespace {
		return s
	}
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' && (c < 0x0B || c > 0x0D) {
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		return ""
	}
	return unsafe.String(&result[0], len(result))
}

// ComputeLineHash computes a 3-letter (a-z) hash for a line. Whitespace is
// stripped before hashing, so only non-whitespace content affects the result.
// Digits are never used so line hashes do not look like line numbers.
func ComputeLineHash(line string) string {
	if line != "" && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	line = removeWhitespace(line)
	h := fnv.New64a()
	_, _ = h.Write([]byte(line))
	n := h.Sum64() % lineHashMod
	out := make([]byte, LineHashLen)
	for i := LineHashLen - 1; i >= 0; i-- {
		out[i] = byte('a' + n%26)
		n /= 26
	}
	return string(out)
}

// NormalizeFileHashText trims trailing spaces/tabs/CR on every line so CRLF
// endings and display-trimmed lines do not invalidate a file tag.
func NormalizeFileHashText(text string) string {
	if text == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(text))
	start := 0
	for i := 0; i <= len(text); i++ {
		if i != len(text) && text[i] != '\n' {
			continue
		}
		line := text[start:i]
		for line != "" {
			c := line[len(line)-1]
			if c == ' ' || c == '\t' || c == '\r' {
				line = line[:len(line)-1]
				continue
			}
			break
		}
		b.WriteString(line)
		if i < len(text) {
			b.WriteByte('\n')
		}
		start = i + 1
	}
	return b.String()
}

// ComputeFileHash returns a 4-digit uppercase hex fingerprint of the whole
// file text (after LF normalization of callers and NormalizeFileHashText).
func ComputeFileHash(text string) string {
	normalized := NormalizeFileHashText(text)
	h := fnv.New64a()
	_, _ = h.Write([]byte(normalized))
	low16 := uint16(h.Sum64() & 0xffff) //nolint:gosec // intentional low-16 fingerprint
	return fmt.Sprintf("%04X", low16)
}

// FormatFileHeader formats the @file path#TAG line shown by read/grep/edit.
func FormatFileHeader(relPath, tag string) string {
	relPath = filepath.ToSlash(strings.TrimSpace(relPath))
	if relPath == "" {
		relPath = "."
	}
	return fmt.Sprintf("@file %s#%s", relPath, strings.ToUpper(strings.TrimSpace(tag)))
}

// ValidateHash checks whether the hash at the given 1-based line matches.
func ValidateHash(line int, hash string, fileContent []string) bool {
	if line < 1 || line > len(fileContent) {
		return false
	}
	expected := strings.ToLower(hash)
	actual := ComputeLineHash(fileContent[line-1])
	return expected == actual
}
