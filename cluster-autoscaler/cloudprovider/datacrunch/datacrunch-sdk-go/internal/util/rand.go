package util

import (
	"crypto/rand"
	"fmt"
	"math/big"
	mathrand "math/rand"
	"time"
)

// SeededRand is a global random instance that is seeded (for non-cryptographic use)
var SeededRand = mathrand.New(mathrand.NewSource(time.Now().UnixNano()))

// GenerateRandomString generates a random string of the specified length using alphanumeric characters
func GenerateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	return GenerateRandomStringWithCharset(length, charset)
}

// GenerateRandomStringWithCharset generates a random string using the provided character set
func GenerateRandomStringWithCharset(length int, charset string) (string, error) {
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
func GenerateRandomHex(length int) (string, error) {
	const hexCharset = "0123456789abcdef"
	return GenerateRandomStringWithCharset(length, hexCharset)
}

// GenerateRandomBytes generates random bytes
func GenerateRandomBytes(length int) ([]byte, error) {
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
func GenerateRandomInt(min, max int64) (int64, error) {
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

// GenerateUUID generates a UUID v4 (random)
func GenerateUUID() (string, error) {
	bytes, err := GenerateRandomBytes(16)
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
func GenerateRandomDuration(min, max time.Duration) (time.Duration, error) {
	if min > max {
		return 0, fmt.Errorf("min cannot be greater than max")
	}

	if min == max {
		return min, nil
	}

	diff := int64(max - min)
	randomValue, err := GenerateRandomInt(0, diff)
	if err != nil {
		return 0, err
	}

	return min + time.Duration(randomValue), nil
}

// GenerateRequestID generates a random request ID suitable for API calls
func GenerateRequestID() (string, error) {
	timestamp := time.Now().Unix()
	randomPart, err := GenerateRandomHex(16)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("req_%x_%s", timestamp, randomPart), nil
}
