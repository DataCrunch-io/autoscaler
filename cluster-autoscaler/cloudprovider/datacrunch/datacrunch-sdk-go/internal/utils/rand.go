package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// RandomGenerator provides cryptographically secure random number generation
type RandomGenerator struct{}

// GenerateRandomString generates a random string of the specified length using alphanumeric characters
func (r *RandomGenerator) GenerateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	return r.GenerateRandomStringWithCharset(length, charset)
}

// GenerateRandomStringWithCharset generates a random string using the provided character set
func (r *RandomGenerator) GenerateRandomStringWithCharset(length int, charset string) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("length must be positive")
	}
	
	if len(charset) == 0 {
		return "", fmt.Errorf("charset cannot be empty")
	}
	
	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))
	
	for i := 0; i < length; i++ {
		randomIndex, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %v", err)
		}
		result[i] = charset[randomIndex.Int64()]
	}
	
	return string(result), nil
}

// GenerateRandomHex generates a random hexadecimal string
func (r *RandomGenerator) GenerateRandomHex(length int) (string, error) {
	const hexCharset = "0123456789abcdef"
	return r.GenerateRandomStringWithCharset(length, hexCharset)
}

// GenerateRandomNumeric generates a random numeric string
func (r *RandomGenerator) GenerateRandomNumeric(length int) (string, error) {
	const numericCharset = "0123456789"
	return r.GenerateRandomStringWithCharset(length, numericCharset)
}

// GenerateRandomAlphabetic generates a random alphabetic string (a-z, A-Z)
func (r *RandomGenerator) GenerateRandomAlphabetic(length int) (string, error) {
	const alphabeticCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	return r.GenerateRandomStringWithCharset(length, alphabeticCharset)
}

// GenerateRandomBytes generates random bytes
func (r *RandomGenerator) GenerateRandomBytes(length int) ([]byte, error) {
	if length <= 0 {
		return nil, fmt.Errorf("length must be positive")
	}
	
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %v", err)
	}
	
	return bytes, nil
}

// GenerateRandomInt generates a random integer between min and max (inclusive)
func (r *RandomGenerator) GenerateRandomInt(min, max int64) (int64, error) {
	if min > max {
		return 0, fmt.Errorf("min cannot be greater than max")
	}
	
	if min == max {
		return min, nil
	}
	
	diff := max - min + 1
	randomValue, err := rand.Int(rand.Reader, big.NewInt(diff))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random integer: %v", err)
	}
	
	return min + randomValue.Int64(), nil
}

// GenerateRandomFloat generates a random float64 between 0.0 and 1.0
func (r *RandomGenerator) GenerateRandomFloat() (float64, error) {
	// Generate a random 53-bit integer for float64 precision
	randomInt, err := rand.Int(rand.Reader, big.NewInt(1<<53))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random float: %v", err)
	}
	
	return float64(randomInt.Int64()) / float64(1<<53), nil
}

// GenerateRandomFloatRange generates a random float64 between min and max
func (r *RandomGenerator) GenerateRandomFloatRange(min, max float64) (float64, error) {
	if min > max {
		return 0, fmt.Errorf("min cannot be greater than max")
	}
	
	randomFloat, err := r.GenerateRandomFloat()
	if err != nil {
		return 0, err
	}
	
	return min + randomFloat*(max-min), nil
}

// GenerateUUID generates a UUID v4 (random)
func (r *RandomGenerator) GenerateUUID() (string, error) {
	bytes, err := r.GenerateRandomBytes(16)
	if err != nil {
		return "", err
	}
	
	// Set version (4) and variant bits
	bytes[6] = (bytes[6] & 0x0f) | 0x40 // Version 4
	bytes[8] = (bytes[8] & 0x3f) | 0x80 // Variant 10
	
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		bytes[0:4],
		bytes[4:6],
		bytes[6:8],
		bytes[8:10],
		bytes[10:16]), nil
}

// GenerateRandomDuration generates a random duration between min and max
func (r *RandomGenerator) GenerateRandomDuration(min, max time.Duration) (time.Duration, error) {
	if min > max {
		return 0, fmt.Errorf("min cannot be greater than max")
	}
	
	if min == max {
		return min, nil
	}
	
	diff := int64(max - min)
	randomValue, err := r.GenerateRandomInt(0, diff)
	if err != nil {
		return 0, err
	}
	
	return min + time.Duration(randomValue), nil
}

// GenerateRequestID generates a random request ID suitable for API calls
func (r *RandomGenerator) GenerateRequestID() (string, error) {
	timestamp := time.Now().Unix()
	randomPart, err := r.GenerateRandomHex(16)
	if err != nil {
		return "", err
	}
	
	return fmt.Sprintf("req_%x_%s", timestamp, randomPart), nil
}

// GenerateInstanceName generates a random instance name with a prefix
func (r *RandomGenerator) GenerateInstanceName(prefix string) (string, error) {
	if prefix == "" {
		prefix = "instance"
	}
	
	randomSuffix, err := r.GenerateRandomString(8)
	if err != nil {
		return "", err
	}
	
	return fmt.Sprintf("%s-%s", prefix, randomSuffix), nil
}

// GeneratePassword generates a cryptographically secure password
func (r *RandomGenerator) GeneratePassword(length int, includeSymbols bool) (string, error) {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if includeSymbols {
		charset += "!@#$%^&*()_+-=[]{}|;:,.<>?"
	}
	
	return r.GenerateRandomStringWithCharset(length, charset)
}

// GenerateJitter generates a random jitter for retry delays
func (r *RandomGenerator) GenerateJitter(base time.Duration, maxJitter time.Duration) (time.Duration, error) {
	if maxJitter <= 0 {
		return base, nil
	}
	
	jitterValue, err := r.GenerateRandomDuration(0, maxJitter)
	if err != nil {
		return base, err
	}
	
	return base + jitterValue, nil
}

// ShuffleSlice shuffles a slice of interfaces in place using Fisher-Yates algorithm
func (r *RandomGenerator) ShuffleSlice(slice []interface{}) error {
	for i := len(slice) - 1; i > 0; i-- {
		j, err := r.GenerateRandomInt(0, int64(i))
		if err != nil {
			return err
		}
		slice[i], slice[j] = slice[j], slice[i]
	}
	return nil
}

// ShuffleStringSlice shuffles a slice of strings in place
func (r *RandomGenerator) ShuffleStringSlice(slice []string) error {
	for i := len(slice) - 1; i > 0; i-- {
		j, err := r.GenerateRandomInt(0, int64(i))
		if err != nil {
			return err
		}
		slice[i], slice[j] = slice[j], slice[i]
	}
	return nil
}

// PickRandom picks a random element from a slice
func (r *RandomGenerator) PickRandom(slice []interface{}) (interface{}, error) {
	if len(slice) == 0 {
		return nil, fmt.Errorf("slice is empty")
	}
	
	if len(slice) == 1 {
		return slice[0], nil
	}
	
	index, err := r.GenerateRandomInt(0, int64(len(slice)-1))
	if err != nil {
		return nil, err
	}
	
	return slice[index], nil
}

// PickRandomString picks a random string from a slice of strings
func (r *RandomGenerator) PickRandomString(slice []string) (string, error) {
	if len(slice) == 0 {
		return "", fmt.Errorf("slice is empty")
	}
	
	if len(slice) == 1 {
		return slice[0], nil
	}
	
	index, err := r.GenerateRandomInt(0, int64(len(slice)-1))
	if err != nil {
		return "", err
	}
	
	return slice[index], nil
}

// Global instance for convenience
var Random = &RandomGenerator{}