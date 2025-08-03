package utils

import (
	"strings"
	"unicode"
)

// StringUtils provides utility functions for string manipulation
type StringUtils struct{}

// ToSnakeCase converts a camelCase or PascalCase string to snake_case
func (s *StringUtils) ToSnakeCase(str string) string {
	var result strings.Builder
	
	for i, r := range str {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}

// ToCamelCase converts a snake_case string to camelCase
func (s *StringUtils) ToCamelCase(str string) string {
	words := strings.Split(str, "_")
	if len(words) == 0 {
		return str
	}
	
	var result strings.Builder
	result.WriteString(strings.ToLower(words[0]))
	
	for _, word := range words[1:] {
		if len(word) > 0 {
			result.WriteString(strings.ToUpper(string(word[0])))
			if len(word) > 1 {
				result.WriteString(strings.ToLower(word[1:]))
			}
		}
	}
	
	return result.String()
}

// ToPascalCase converts a snake_case string to PascalCase
func (s *StringUtils) ToPascalCase(str string) string {
	words := strings.Split(str, "_")
	var result strings.Builder
	
	for _, word := range words {
		if len(word) > 0 {
			result.WriteString(strings.ToUpper(string(word[0])))
			if len(word) > 1 {
				result.WriteString(strings.ToLower(word[1:]))
			}
		}
	}
	
	return result.String()
}

// IsEmpty checks if a string is empty or contains only whitespace
func (s *StringUtils) IsEmpty(str string) bool {
	return strings.TrimSpace(str) == ""
}

// DefaultIfEmpty returns the default value if the string is empty
func (s *StringUtils) DefaultIfEmpty(str, defaultValue string) string {
	if s.IsEmpty(str) {
		return defaultValue
	}
	return str
}

// Truncate truncates a string to the specified length
func (s *StringUtils) Truncate(str string, length int) string {
	if len(str) <= length {
		return str
	}
	return str[:length]
}

// TruncateWithEllipsis truncates a string and adds ellipsis if truncated
func (s *StringUtils) TruncateWithEllipsis(str string, length int) string {
	if len(str) <= length {
		return str
	}
	if length <= 3 {
		return s.Truncate(str, length)
	}
	return str[:length-3] + "..."
}

// Contains checks if a string contains any of the provided substrings
func (s *StringUtils) Contains(str string, substrings ...string) bool {
	for _, substring := range substrings {
		if strings.Contains(str, substring) {
			return true
		}
	}
	return false
}

// ContainsAny checks if a string contains any of the provided substrings (case-insensitive)
func (s *StringUtils) ContainsAnyIgnoreCase(str string, substrings ...string) bool {
	lowerStr := strings.ToLower(str)
	for _, substring := range substrings {
		if strings.Contains(lowerStr, strings.ToLower(substring)) {
			return true
		}
	}
	return false
}

// SplitAndTrim splits a string and trims whitespace from each part
func (s *StringUtils) SplitAndTrim(str, separator string) []string {
	parts := strings.Split(str, separator)
	result := make([]string, 0, len(parts))
	
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	
	return result
}

// Join joins strings with a separator, ignoring empty strings
func (s *StringUtils) JoinNonEmpty(separator string, strs ...string) string {
	var nonEmpty []string
	for _, str := range strs {
		if !s.IsEmpty(str) {
			nonEmpty = append(nonEmpty, str)
		}
	}
	return strings.Join(nonEmpty, separator)
}

// Global instance for convenience
var Strings = &StringUtils{}