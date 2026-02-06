package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "importer_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &Config{
		CredentialsFile: "test_credentials.json",
		TokenFile:       "test_token.json",
		InputDir:        tempDir,
		ParallelWorkers: 3,
		PreserveDates:   true,
		Limit:           0,
	}

	// This will fail because we don't have valid credentials, but we can test validation
	_, err = New(config)
	if err == nil {
		t.Error("Expected error for invalid credentials file")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				InputDir:        ".",
				ParallelWorkers: 3,
				Limit:           0,
			},
			expectError: false,
		},
		{
			name: "missing input dir",
			config: &Config{
				ParallelWorkers: 3,
			},
			expectError: true,
		},
		{
			name: "negative parallel workers",
			config: &Config{
				InputDir:        ".",
				ParallelWorkers: -1,
			},
			expectError: true,
		},
		{
			name: "negative limit",
			config: &Config{
				InputDir:        ".",
				ParallelWorkers: 3,
				Limit:           -1,
			},
			expectError: true,
		},
		{
			name: "non-existent input dir",
			config: &Config{
				InputDir:        "/non/existent/path",
				ParallelWorkers: 3,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestExtractMessageID(t *testing.T) {
	importer := &Importer{}

	tests := []struct {
		name       string
		emailData  []byte
		expectedID string
	}{
		{
			name:       "simple message-id",
			emailData:  []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\nMessage-ID: <test@example.com>\n\nBody"),
			expectedID: "test@example.com",
		},
		{
			name:       "message-id with parameters",
			emailData:  []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\nMessage-ID: <test@example.com; Mon, 05 Feb 2026 12:00:00 GMT>\n\nBody"),
			expectedID: "test@example.com",
		},
		{
			name:       "message-id with angle brackets and space",
			emailData:  []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\nMessage-ID: <test@example.com> \n\nBody"),
			expectedID: "test@example.com",
		},
		{
			name:       "no message-id header",
			emailData:  []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\n\nBody"),
			expectedID: "",
		},
		{
			name:       "lowercase message-id",
			emailData:  []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\nmessage-id: <test@example.com>\n\nBody"),
			expectedID: "test@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := importer.extractMessageID(tt.emailData)
			if result != tt.expectedID {
				t.Errorf("extractMessageID() = %q, want %q", result, tt.expectedID)
			}
		})
	}
}

func TestNormalizeLabelName(t *testing.T) {
	importer := &Importer{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "system label Important",
			input:    "Important",
			expected: "IMPORTANT",
		},
		{
			name:     "system label Starred",
			input:    "Starred",
			expected: "STARRED",
		},
		{
			name:     "system label Unread",
			input:    "Unread",
			expected: "UNREAD",
		},
		{
			name:     "Category Personal (system-managed)",
			input:    "Category Personal",
			expected: "", // Gmail categories are system-managed and cannot be set via API
		},
		{
			name:     "Category Social (system-managed)",
			input:    "Category Social",
			expected: "", // Gmail categories are system-managed and cannot be set via API
		},
		{
			name:     "Category Updates (system-managed)",
			input:    "Category Updates",
			expected: "", // Gmail categories are system-managed and cannot be set via API
		},
		{
			name:     "Category Forums (system-managed)",
			input:    "Category Forums",
			expected: "", // Gmail categories are system-managed and cannot be set via API
		},
		{
			name:     "Category Purchases (system-managed)",
			input:    "Category Purchases",
			expected: "", // Gmail categories are system-managed and cannot be set via API
		},
		{
			name:     "Category Promotions (system-managed)",
			input:    "Category Promotions",
			expected: "", // Gmail categories are system-managed and cannot be set via API
		},
		{
			name:     "user-defined label",
			input:    "my-custom-label",
			expected: "my-custom-label",
		},
		{
			name:     "user label with spaces",
			input:    "projects/my project",
			expected: "projects/my project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := importer.normalizeLabelName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeLabelName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractLabelsFromHeaders(t *testing.T) {
	importer := &Importer{}

	tests := []struct {
		name           string
		emailData      []byte
		expectedLabels []string
	}{
		{
			name:           "single label",
			emailData:      []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\nX-Gmail-Labels: Important\n\nBody"),
			expectedLabels: []string{"Important"},
		},
		{
			name:           "multiple labels",
			emailData:      []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\nX-Gmail-Labels: Important,Starred,Unread\n\nBody"),
			expectedLabels: []string{"Important", "Starred", "Unread"},
		},
		{
			name:           "labels with spaces",
			emailData:      []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\nX-Gmail-Labels: Category Personal, Category Social\n\nBody"),
			expectedLabels: []string{"Category Personal", "Category Social"},
		},
		{
			name:           "labels with whitespace",
			emailData:      []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\nX-Gmail-Labels:  Important  ,  Starred  \n\nBody"),
			expectedLabels: []string{"Important", "Starred"},
		},
		{
			name:           "no labels header",
			emailData:      []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\n\nBody"),
			expectedLabels: nil,
		},
		{
			name:           "empty labels header",
			emailData:      []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\nX-Gmail-Labels: \n\nBody"),
			expectedLabels: nil,
		},
		{
			name:           "user-defined labels",
			emailData:      []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\nX-Gmail-Labels: my-label,projects/work\n\nBody"),
			expectedLabels: []string{"my-label", "projects/work"},
		},
		{
			name:           "mixed system and user labels",
			emailData:      []byte("From: sender@example.com\nTo: recipient@example.com\nSubject: Test\nX-Gmail-Labels: Important,my-label,Category Personal\n\nBody"),
			expectedLabels: []string{"Important", "my-label", "Category Personal"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := importer.extractLabelsFromHeaders(tt.emailData)
			if len(result) != len(tt.expectedLabels) {
				t.Errorf("Expected %d labels, got %d: %v vs %v", len(tt.expectedLabels), len(result), tt.expectedLabels, result)
			}
			for i, label := range tt.expectedLabels {
				if i >= len(result) || result[i] != label {
					t.Errorf("Label %d: expected %q, got %q", i, label, result)
				}
			}
		})
	}
}

func TestFindEmailFiles(t *testing.T) {
	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "importer_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := []string{
		"email1.eml",
		"email2.json",
		"email3.mbox",
		"not_email.txt",
		"document.pdf",
	}

	for _, filename := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filePath, []byte("test content"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Create subdirectory with more files
	subDir := filepath.Join(tempDir, "subdir")
	err = os.MkdirAll(subDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	subFile := filepath.Join(subDir, "email4.eml")
	err = os.WriteFile(subFile, []byte("test content"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	// Create importer instance
	importer := &Importer{
		config: &Config{
			InputDir: tempDir,
		},
	}

	// Test finding email files
	emailFiles, err := importer.findEmailFiles()
	if err != nil {
		t.Fatalf("Failed to find email files: %v", err)
	}

	// Should find 4 email files (3 in root + 1 in subdir)
	expectedCount := 4
	if len(emailFiles) != expectedCount {
		t.Errorf("Expected %d email files, got %d", expectedCount, len(emailFiles))
	}

	// Check that only email files are included
	for _, filePath := range emailFiles {
		ext := filepath.Ext(filePath)
		if ext != ".eml" && ext != ".json" && ext != ".mbox" {
			t.Errorf("Unexpected file extension found: %s", ext)
		}
	}
}

func TestEncodeBase64URL(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "simple text",
			input:    []byte("hello world"),
			expected: "aGVsbG8gd29ybGQ",
		},
		{
			name:     "empty input",
			input:    []byte(""),
			expected: "",
		},
		{
			name:     "binary data",
			input:    []byte{0x00, 0x01, 0x02, 0x03},
			expected: "AAECAw",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encodeBase64URL(tt.input)
			if result != tt.expected {
				t.Errorf("encodeBase64URL() = %s, want %s", result, tt.expected)
			}
		})
	}
}
