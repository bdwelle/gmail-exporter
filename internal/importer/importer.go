package importer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/api/gmail/v1"

	"github.com/octasoft-ltd/gmail-exporter/internal/auth"
	"github.com/octasoft-ltd/gmail-exporter/internal/metrics"
)

// Config represents the importer configuration
type Config struct {
	CredentialsFile    string `json:"credentials_file"`
	TokenFile          string `json:"token_file"`
	InputDir           string `json:"input_dir"`
	ParallelWorkers    int    `json:"parallel_workers"`
	PreserveDates      bool   `json:"preserve_dates"`
	Limit              int    `json:"limit"`
	Labels             string `json:"labels"`
	SkipDuplicates     bool   `json:"skip_duplicates"`
	SkipInboxLabel     bool   `json:"skip_inbox_label"`
	SkipImportantLabel bool   `json:"skip_important_label"`
	SkipStarredLabel   bool   `json:"skip_starred_label"`
}

// Result represents the import operation result
type Result struct {
	TotalFound    int           `json:"total_found"`
	TotalImported int           `json:"total_imported"`
	TotalFailed   int           `json:"total_failed"`
	TotalSkipped  int           `json:"total_skipped"`
	TotalSize     int64         `json:"total_size"`
	Duration      time.Duration `json:"duration"`
	Failures      []Failure     `json:"failures,omitempty"`
}

// Failure represents a failed import operation
type Failure struct {
	FilePath  string    `json:"file_path"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// Importer handles email import operations
type Importer struct {
	config           *Config
	authenticator    *auth.Authenticator
	gmailService     *gmail.Service
	metrics          *metrics.Collector
	labelIds         []string
	labelCache       map[string]string
	labelCacheMutex  sync.RWMutex
	allLabelsFetched bool
}

// New creates a new importer instance
func New(config *Config) (*Importer, error) {
	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create authenticator
	authenticator, err := auth.NewAuthenticator(config.CredentialsFile, config.TokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticator: %w", err)
	}

	// Get Gmail service
	gmailService, err := authenticator.GetGmailService()
	if err != nil {
		return nil, fmt.Errorf("failed to get Gmail service: %w", err)
	}

	// Create metrics collector
	metricsCollector := metrics.NewCollector("import")

	return &Importer{
		config:        config,
		authenticator: authenticator,
		gmailService:  gmailService,
		metrics:       metricsCollector,
	}, nil
}

// Import performs the email import operation
func (i *Importer) Import() (*Result, error) {
	startTime := time.Now()
	i.metrics.Start()

	logrus.WithFields(logrus.Fields{
		"input_dir": i.config.InputDir,
		"limit":     i.config.Limit,
	}).Info("Starting email import")

	// Find email files
	emailFiles, err := i.findEmailFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to find email files: %w", err)
	}

	logrus.WithField("count", len(emailFiles)).Info("Found email files to import")

	// Initialize label cache
	i.labelCache = make(map[string]string)

	// Apply limit if specified
	if i.config.Limit > 0 && len(emailFiles) > i.config.Limit {
		emailFiles = emailFiles[:i.config.Limit]
		logrus.WithField("limited_count", len(emailFiles)).Info("Limited number of files to process")
	}

	// Import emails
	result, err := i.importEmails(emailFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to import emails: %w", err)
	}

	// Calculate duration
	result.Duration = time.Since(startTime)
	result.TotalFound = len(emailFiles)

	// Record metrics
	i.metrics.RecordEmailsProcessed(result.TotalImported, result.TotalFailed)
	i.metrics.RecordBytesProcessed(result.TotalSize)
	i.metrics.RecordDuration(result.Duration)
	i.metrics.SetTotalMatched(result.TotalFound)

	// Save metrics
	metricsPath := filepath.Join(filepath.Dir(i.config.InputDir), "import_metrics.json")
	if err := i.metrics.Save(metricsPath); err != nil {
		logrus.WithError(err).Warn("Failed to save metrics")
	}

	logrus.WithFields(logrus.Fields{
		"total_found":    result.TotalFound,
		"total_imported": result.TotalImported,
		"total_failed":   result.TotalFailed,
		"duration":       result.Duration,
	}).Info("Import completed")

	return result, nil
}

// findEmailFiles finds all email files in the input directory
func (i *Importer) findEmailFiles() ([]string, error) {
	var emailFiles []string

	err := filepath.WalkDir(i.config.InputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Check for supported email file extensions
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".eml" || ext == ".json" || ext == ".mbox" {
			emailFiles = append(emailFiles, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return emailFiles, nil
}

// importEmails imports the specified email files
func (i *Importer) importEmails(emailFiles []string) (*Result, error) {
	result := &Result{
		Failures: make([]Failure, 0),
	}

	// Create worker pool for parallel processing
	if i.config.ParallelWorkers <= 0 {
		i.config.ParallelWorkers = 1
	}

	jobs := make(chan string, len(emailFiles))
	results := make(chan importResult, len(emailFiles))

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < i.config.ParallelWorkers; w++ {
		wg.Add(1)
		go i.importWorker(jobs, results, &wg)
	}

	// Send jobs
	for _, filePath := range emailFiles {
		jobs <- filePath
	}
	close(jobs)

	// Wait for workers to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results with progress indicator
	processed := 0
	total := len(emailFiles)
	for importRes := range results {
		processed++

		if importRes.Error != nil {
			result.TotalFailed++
			result.Failures = append(result.Failures, Failure{
				FilePath:  importRes.FilePath,
				Error:     importRes.Error.Error(),
				Timestamp: time.Now(),
			})
			logrus.WithError(importRes.Error).WithField("file_path", importRes.FilePath).Error("Failed to import email")
		} else if importRes.Size == 0 {
			// Email was skipped (e.g., duplicate)
			result.TotalSkipped++
			logrus.WithField("file_path", importRes.FilePath).Debug("Skipped email")
		} else {
			result.TotalImported++
			result.TotalSize += importRes.Size
		}

		// Show progress (imported + skipped)
		completed := result.TotalImported + result.TotalSkipped
		fmt.Printf("\rProgress: %d of %d messages processed (%.1f%%)",
			completed, total, float64(processed)/float64(total)*100)
	}
	fmt.Println() // New line after progress

	return result, nil
}

// importResult represents the result of importing a single email
type importResult struct {
	FilePath string
	Size     int64
	Error    error
}

// importWorker is a worker function for importing emails in parallel
func (i *Importer) importWorker(jobs <-chan string, results chan<- importResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for filePath := range jobs {
		size, err := i.importSingleEmail(filePath)
		results <- importResult{
			FilePath: filePath,
			Size:     size,
			Error:    err,
		}
	}
}

// importSingleEmail imports a single email file
func (i *Importer) importSingleEmail(filePath string) (int64, error) {
	// Read the email file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	// Determine file type and process accordingly
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".eml":
		return i.importEMLFile(data)
	case ".json":
		return i.importJSONFile(data)
	case ".mbox":
		return i.importMboxFile(data)
	default:
		return 0, fmt.Errorf("unsupported file type: %s", ext)
	}
}

// importEMLFile imports an EML format email
func (i *Importer) importEMLFile(data []byte) (int64, error) {
	// Extract labels from email headers
	labelIds := i.getLabelIdsForEmail(data)

	// Check for duplicates if enabled
	if i.config.SkipDuplicates {
		messageID := i.extractMessageID(data)
		if messageID != "" {
			exists, err := i.messageExists(messageID)
			if err != nil {
				logrus.WithError(err).WithField("message_id", messageID).Warn("Failed to check if message exists, proceeding with import")
			} else if exists {
				logrus.WithField("message_id", messageID).Info("Message already exists in Gmail, skipping")
				return 0, nil
			}
		}
	}

	// Create a Gmail message from the EML data
	message := &gmail.Message{
		Raw:      encodeBase64URL(data),
		LabelIds: labelIds,
	}

	// Import the message (does not send, just adds to mailbox)
	_, err := i.gmailService.Users.Messages.Import("me", message).Do()
	if err != nil {
		return 0, fmt.Errorf("failed to import message: %w", err)
	}

	return int64(len(data)), nil
}

// importJSONFile imports a JSON format email
func (i *Importer) importJSONFile(data []byte) (int64, error) {
	// Parse the JSON to extract the raw email data
	var emailData struct {
		Raw string `json:"raw"`
	}

	if err := json.Unmarshal(data, &emailData); err != nil {
		return 0, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Extract labels from email headers
	rawBytes, err := base64.URLEncoding.DecodeString(emailData.Raw + strings.Repeat("=", ((4-len(emailData.Raw)%4)%4)))
	if err != nil {
		// If decoding fails, just use the raw as-is
		rawBytes = []byte(emailData.Raw)
	}

	// Check for duplicates if enabled
	if i.config.SkipDuplicates {
		messageID := i.extractMessageID(rawBytes)
		if messageID != "" {
			exists, err := i.messageExists(messageID)
			if err != nil {
				logrus.WithError(err).WithField("message_id", messageID).Warn("Failed to check if message exists, proceeding with import")
			} else if exists {
				logrus.WithField("message_id", messageID).Info("Message already exists in Gmail, skipping")
				return 0, nil
			}
		}
	}

	labelIds := i.getLabelIdsForEmail(rawBytes)

	// Create a Gmail message
	message := &gmail.Message{
		Raw:      emailData.Raw,
		LabelIds: labelIds,
	}

	// Import the message (does not send, just adds to mailbox)
	_, err = i.gmailService.Users.Messages.Import("me", message).Do()
	if err != nil {
		return 0, fmt.Errorf("failed to import message: %w", err)
	}

	return int64(len(data)), nil
}

// importMboxFile imports an mbox format email
func (i *Importer) importMboxFile(data []byte) (int64, error) {
	// For mbox files, we need to parse the format and extract individual messages
	// This is a simplified implementation - in practice, you'd want a proper mbox parser
	message := &gmail.Message{
		Raw:      encodeBase64URL(data),
		LabelIds: i.getLabelIdsForEmail(data),
	}

	// Import the message (does not send, just adds to mailbox)
	_, err := i.gmailService.Users.Messages.Import("me", message).Do()
	if err != nil {
		return 0, fmt.Errorf("failed to import message: %w", err)
	}

	return int64(len(data)), nil
}

// getLabelIdsForEmail extracts labels from email headers and resolves them to IDs
func (i *Importer) getLabelIdsForEmail(data []byte) []string {
	// Check if --labels flag is set (command-line labels)
	if i.config.Labels != "" {
		logrus.WithField("labels", i.config.Labels).Debug("Using --labels flag")
		// Use command-line labels (cached)
		if len(i.labelIds) == 0 {
			i.labelCacheMutex.Lock()
			if len(i.labelIds) == 0 {
				// Resolve labels from command line
				labelNames := strings.Split(i.config.Labels, ",")
				for _, name := range labelNames {
					name = strings.TrimSpace(name)
					if name == "" {
						continue
					}
					if labelId := i.resolveLabelName(name); labelId != "" {
						i.labelIds = append(i.labelIds, labelId)
					}
				}
			}
			i.labelCacheMutex.Unlock()
		}
		return i.labelIds
	}

	// Extract labels from X-Gmail-Labels header
	emailLabels := i.extractLabelsFromHeaders(data)
	if len(emailLabels) == 0 {
		return nil
	}

	// Resolve label names to IDs
	labelIds := make([]string, 0, len(emailLabels))
	for _, labelName := range emailLabels {
		labelName = strings.TrimSpace(labelName)
		if labelName == "" {
			continue
		}
		if labelId := i.resolveLabelName(labelName); labelId != "" {
			labelIds = append(labelIds, labelId)
		}
	}

	logrus.WithFields(logrus.Fields{"email_labels": emailLabels, "resolved_count": len(labelIds)}).Debug("Processed email labels")
	return labelIds
}

// extractLabelsFromHeaders extracts label names from the X-Gmail-Labels header
func (i *Importer) extractLabelsFromHeaders(data []byte) []string {
	dataStr := string(data)

	// Find the X-Gmail-Labels header
	headerPrefix := "\nX-Gmail-Labels:"
	idx := strings.Index(dataStr, headerPrefix)
	if idx == -1 {
		return nil
	}

	// Extract the header value (up to the next line that doesn't start with whitespace)
	start := idx + len(headerPrefix)
	end := len(dataStr)

	// Find the end of the header value (next line that doesn't start with whitespace)
	for i := start; i < len(dataStr); i++ {
		if dataStr[i] == '\n' && (i+1 >= len(dataStr) || (dataStr[i+1] != ' ' && dataStr[i+1] != '\t' && dataStr[i+1] != '\r')) {
			end = i
			break
		}
	}

	// Extract and parse the label list
	labelValue := strings.TrimSpace(dataStr[start:end])
	if labelValue == "" {
		return nil
	}

	// Split by comma and trim whitespace
	labels := strings.Split(labelValue, ",")
	result := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label != "" {
			result = append(result, label)
		}
	}

	return result
}

// normalizeLabelName normalizes label names from X-Gmail-Labels header to Gmail API format
func (i *Importer) normalizeLabelName(labelName string) string {
	// Common mappings from X-Gmail-Labels to Gmail API
	mappings := map[string]string{
		"Important":           "IMPORTANT",
		"Starred":             "STARRED",
		"Unread":              "UNREAD",
		"Category Personal":   "CATEGORY_PERSONAL",
		"Category Social":     "CATEGORY_SOCIAL",
		"Category Updates":    "CATEGORY_UPDATES",
		"Category Forums":     "CATEGORY_FORUMS",
		"Category Promotions": "CATEGORY_PROMOTIONS",
	}

	// Check for known mappings
	if mapped, ok := mappings[labelName]; ok {
		return mapped
	}

	// Convert "Category XXX" to "CATEGORY_XXX"
	if strings.HasPrefix(labelName, "Category ") {
		category := strings.TrimPrefix(labelName, "Category ")
		category = strings.ToUpper(strings.ReplaceAll(category, " ", "_"))
		return "CATEGORY_" + category
	}

	// Try uppercase for system labels
	upper := strings.ToUpper(labelName)
	if upper == "IMPORTANT" || upper == "STARRED" || upper == "INBOX" ||
		upper == "SENT" || upper == "DRAFT" || upper == "SPAM" ||
		upper == "TRASH" || upper == "UNREAD" || upper == "CHAT" {
		return upper
	}

	// Return as-is for user-defined labels
	return labelName
}

// resolveLabelName resolves a single label name to its ID (with caching)
func (i *Importer) resolveLabelName(labelName string) string {
	// Skip labels if configured
	upper := strings.ToUpper(labelName)
	if upper == "INBOX" && i.config.SkipInboxLabel {
		logrus.WithField("label", labelName).Debug("Skipping Inbox label per configuration")
		return ""
	}
	if upper == "IMPORTANT" && i.config.SkipImportantLabel {
		logrus.WithField("label", labelName).Debug("Skipping Important label per configuration")
		return ""
	}
	if upper == "STARRED" && i.config.SkipStarredLabel {
		logrus.WithField("label", labelName).Debug("Skipping Starred label per configuration")
		return ""
	}

	// Normalize label name first
	normalizedName := i.normalizeLabelName(labelName)

	// Check cache first
	i.labelCacheMutex.RLock()
	if labelId, ok := i.labelCache[normalizedName]; ok {
		i.labelCacheMutex.RUnlock()
		return labelId
	}
	i.labelCacheMutex.RUnlock()

	// Fetch all labels if not already done
	if !i.allLabelsFetched {
		i.labelCacheMutex.Lock()
		if !i.allLabelsFetched {
			labelsResp, err := i.gmailService.Users.Labels.List("me").Do()
			if err != nil {
				i.labelCacheMutex.Unlock()
				logrus.WithError(err).WithField("label", labelName).Warn("Failed to fetch labels, skipping")
				return ""
			}
			logrus.WithField("count", len(labelsResp.Labels)).Debug("Fetched labels from Gmail")
			for _, label := range labelsResp.Labels {
				i.labelCache[label.Name] = label.Id
			}
			i.allLabelsFetched = true
		}
		i.labelCacheMutex.Unlock()
	}

	// Check cache again after fetching
	i.labelCacheMutex.RLock()
	labelId, ok := i.labelCache[normalizedName]
	i.labelCacheMutex.RUnlock()

	if !ok {
		logrus.WithField("label", labelName).Debug("Label not found, skipping")
		return ""
	}

	return labelId
}

// validateConfig validates the importer configuration
func validateConfig(config *Config) error {
	if config.InputDir == "" {
		return fmt.Errorf("input directory is required")
	}

	if _, err := os.Stat(config.InputDir); os.IsNotExist(err) {
		return fmt.Errorf("input directory does not exist: %s", config.InputDir)
	}

	if config.ParallelWorkers < 0 {
		return fmt.Errorf("parallel workers must be >= 0")
	}

	if config.Limit < 0 {
		return fmt.Errorf("limit must be >= 0")
	}

	return nil
}

// extractMessageID extracts the Message-ID header from email data
func (i *Importer) extractMessageID(data []byte) string {
	dataStr := string(data)

	// Find the Message-ID header
	headerPrefix := "\nMessage-ID:"
	idx := strings.Index(dataStr, headerPrefix)
	if idx == -1 {
		// Try with lowercase
		headerPrefix = "\nmessage-id:"
		idx = strings.Index(dataStr, headerPrefix)
		if idx == -1 {
			return ""
		}
	}

	// Extract the header value (up to the next line)
	start := idx + len(headerPrefix)
	end := len(dataStr)

	for i := start; i < len(dataStr); i++ {
		if dataStr[i] == '\n' {
			end = i
			break
		}
	}

	messageID := strings.TrimSpace(dataStr[start:end])

	// Remove angle brackets and any trailing parameters
	messageID = strings.Trim(messageID, "<>")

	// Extract just the ID part before parameters (like ; or space)
	if idx := strings.IndexAny(messageID, ";\t "); idx >= 0 {
		messageID = messageID[:idx]
	}

	messageID = strings.TrimSpace(messageID)

	return messageID
}

// messageExists checks if a message with the given Message-ID already exists in Gmail
func (i *Importer) messageExists(messageID string) (bool, error) {
	if messageID == "" {
		return false, nil
	}

	// Search for message with this Message-ID
	q := fmt.Sprintf("rfc822msgid:%s", messageID)

	listCall := i.gmailService.Users.Messages.List("me").Q(q).MaxResults(1)
	resp, err := listCall.Do()
	if err != nil {
		return false, fmt.Errorf("failed to check if message exists: %w", err)
	}

	// If we get any results, the message exists
	return len(resp.Messages) > 0, nil
}

// encodeBase64URL encodes data in base64url format for Gmail API
func encodeBase64URL(data []byte) string {
	encoded := base64.URLEncoding.EncodeToString(data)
	return strings.TrimRight(encoded, "=")
}
