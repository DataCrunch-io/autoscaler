package credentials

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultSharedCredentialsFilename is the default filename for shared credentials
	DefaultSharedCredentialsFilename = "credentials"
)

// SharedCredentialsProvider retrieves credentials from a shared credentials file
// Similar to AWS ~/.aws/credentials file
type SharedCredentialsProvider struct {
	// Filename is the path to the shared credentials file
	// If empty, will default to ~/.datacrunch/credentials
	Filename string

	// Profile is the profile name to use from the credentials file
	// If empty, will default to "default"
	Profile string

	// Retrieved indicates if the credentials have been loaded
	retrieved bool
}

// NewSharedCredentials returns a new Credentials with SharedCredentialsProvider
func NewSharedCredentials(filename, profile string) *Credentials {
	return NewCredentials(&SharedCredentialsProvider{
		Filename: filename,
		Profile:  profile,
	})
}

// Retrieve retrieves credentials from the shared credentials file
func (s *SharedCredentialsProvider) Retrieve() (Value, error) {
	s.retrieved = false

	filename := s.Filename
	if filename == "" {
		filename = s.defaultFilename()
	}

	profile := s.Profile
	if profile == "" {
		profile = "default"
	}

	creds, err := s.loadCredentials(filename, profile)
	if err != nil {
		return Value{ProviderName: SharedCredentialsProviderName}, err
	}

	s.retrieved = true
	creds.ProviderName = SharedCredentialsProviderName
	return creds, nil
}

// RetrieveWithContext retrieves credentials with context support
func (s *SharedCredentialsProvider) RetrieveWithContext(ctx context.Context) (Value, error) {
	return s.Retrieve()
}

// IsExpired returns false since shared credentials don't expire
func (s *SharedCredentialsProvider) IsExpired() bool {
	return !s.retrieved
}

// defaultFilename returns the default path for the shared credentials file
func (s *SharedCredentialsProvider) defaultFilename() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".datacrunch", DefaultSharedCredentialsFilename)
}

// loadCredentials loads credentials from the specified file and profile
func (s *SharedCredentialsProvider) loadCredentials(filename, profile string) (Value, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Value{}, fmt.Errorf("failed to open credentials file %s: %w", filename, err)
	}
	defer file.Close()

	var (
		creds           Value
		inTargetProfile bool
		scanner         = bufio.NewScanner(file)
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Check for profile headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			profileName := strings.Trim(line, "[]")
			inTargetProfile = profileName == profile
			continue
		}

		// Skip if not in target profile
		if !inTargetProfile {
			continue
		}

		// Parse key-value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
			(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
			value = value[1 : len(value)-1]
		}

		switch strings.ToLower(key) {
		case "client_id", "datacrunch_client_id":
			creds.ClientID = value
		case "client_secret", "datacrunch_client_secret":
			creds.ClientSecret = value
		case "base_url", "datacrunch_base_url":
			creds.BaseURL = value
		case "access_token", "datacrunch_access_token":
			creds.AccessToken = value
		case "refresh_token", "datacrunch_refresh_token":
			creds.RefreshToken = value
		}
	}

	if err := scanner.Err(); err != nil {
		return Value{}, fmt.Errorf("error reading credentials file: %w", err)
	}

	if !creds.HasKeys() {
		return Value{}, fmt.Errorf("profile %s not found or missing required credentials in %s", profile, filename)
	}

	return creds, nil
}
