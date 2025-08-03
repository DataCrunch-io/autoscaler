package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// IOUtils provides utility functions for I/O operations
type IOUtils struct{}

// ReadFileToString reads a file and returns its content as a string
func (u *IOUtils) ReadFileToString(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %v", filename, err)
	}
	return string(data), nil
}

// WriteStringToFile writes a string to a file
func (u *IOUtils) WriteStringToFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

// ReadJSONFile reads a JSON file and unmarshals it into the target struct
func (u *IOUtils) ReadJSONFile(filename string, target interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read JSON file %s: %v", filename, err)
	}
	
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON from %s: %v", filename, err)
	}
	
	return nil
}

// WriteJSONFile marshals a struct to JSON and writes it to a file
func (u *IOUtils) WriteJSONFile(filename string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %v", err)
	}
	
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file %s: %v", filename, err)
	}
	
	return nil
}

// CopyReader copies data from src to dst
func (u *IOUtils) CopyReader(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

// ReadAllFromReader reads all data from a reader
func (u *IOUtils) ReadAllFromReader(reader io.Reader) ([]byte, error) {
	return io.ReadAll(reader)
}

// CreateBuffer creates a new buffer from a byte slice
func (u *IOUtils) CreateBuffer(data []byte) *bytes.Buffer {
	return bytes.NewBuffer(data)
}

// CreateBufferFromString creates a new buffer from a string
func (u *IOUtils) CreateBufferFromString(str string) *bytes.Buffer {
	return bytes.NewBufferString(str)
}

// FileExists checks if a file exists
func (u *IOUtils) FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// DirExists checks if a directory exists
func (u *IOUtils) DirExists(dirname string) bool {
	info, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// CreateDirIfNotExists creates a directory if it doesn't exist
func (u *IOUtils) CreateDirIfNotExists(dirname string) error {
	if !u.DirExists(dirname) {
		return os.MkdirAll(dirname, 0755)
	}
	return nil
}

// GetFileSize returns the size of a file in bytes
func (u *IOUtils) GetFileSize(filename string) (int64, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info for %s: %v", filename, err)
	}
	return info.Size(), nil
}

// IsJSONValid checks if a byte slice contains valid JSON
func (u *IOUtils) IsJSONValid(data []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(data, &js) == nil
}

// PrettyPrintJSON formats JSON data for readable output
func (u *IOUtils) PrettyPrintJSON(data interface{}) (string, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}
	return string(jsonData), nil
}

// CompactJSON removes unnecessary whitespace from JSON
func (u *IOUtils) CompactJSON(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, data); err != nil {
		return nil, fmt.Errorf("failed to compact JSON: %v", err)
	}
	return buffer.Bytes(), nil
}

// Global instance for convenience
var IO = &IOUtils{}