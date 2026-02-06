#!/bin/bash
# Gmail Import Batch Script
# Imports all main Gmail folders from Google Takeout

set -e  # Exit on error

# Configuration
MAIL_DIR="/Volumes/Samsung 2T/GoogleTakeout_2025-11/Mail"
OUTPUT_DIR="2-split-to-import"
IMPORT_DATE=$(date +"%Y-%m-%d")
SPLITTER_SCRIPT="./split-mbox-emails-streaming.py"
IMPORT_CMD="./gmail-exporter"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if files exist
check_files() {
    local file="$1"
    if [ ! -f "$file" ]; then
        log_error "File not found: $file"
        exit 1
    fi
}

# Import a single mbox file
import_mbox() {
    local mbox_file="$1"
    local label_type="$2"
    local extra_flags="$3"

    local filename=$(basename "$mbox_file")
    local import_label="import-${label_type}-${IMPORT_DATE}"

    log_info "Processing: $filename"
    log_info "Import label: $import_label"

    # Check if file exists
    check_files "$mbox_file"

    # Clear output directory and split mbox
    log_info "Splitting $filename..."
    if [ -d "$OUTPUT_DIR" ]; then
        rm -rf "$OUTPUT_DIR"
    fi

    $SPLITTER_SCRIPT "$mbox_file" --clear

    # Import with appropriate flags
    log_info "Importing $filename to Gmail..."
    $IMPORT_CMD import \
        --skip-duplicates \
        --skip-categories \
        $extra_flags \
        --input-dir "$OUTPUT_DIR" \
        --add-label="$import_label"

    log_success "Imported $filename"
    echo ""
}

# Main execution
echo ""
echo "=========================================="
echo "Gmail Import Batch Script"
echo "=========================================="
echo "Import date: $IMPORT_DATE"
echo "Mail directory: $MAIL_DIR"
echo "=========================================="
echo ""
echo "⚠️  IMPORTANT: Before starting import, make sure you've created these labels in Gmail:"
echo "     1. import-inbox-$IMPORT_DATE"
echo "     2. import-sent-$IMPORT_DATE"
echo "     3. import-starred-$IMPORT_DATE"
echo "     4. import-archived-$IMPORT_DATE"
echo ""
echo "💡 How to create labels in Gmail:"
echo "     1. Open Gmail (https://mail.google.com)"
echo "     2. Click 'Labels' on left sidebar"
echo "     3. Click '+ Create' button"
echo "     4. Enter label name (e.g., import-inbox-$IMPORT_DATE)"
echo "     5. Click 'Create'"
echo ""
read -p "Press ENTER to continue once labels are created, or Ctrl+C to exit..."

echo ""
echo "=========================================="
echo ""

# Import Inbox (received emails)
import_mbox \
    "$MAIL_DIR/Inbox.mbox" \
    "inbox" \
    "--no-inbox --no-starred --no-important"

# Import Sent emails
import_mbox \
    "$MAIL_DIR/Sent.mbox" \
    "sent" \
    "--no-inbox"

# Import Starred emails
import_mbox \
    "$MAIL_DIR/Starred.mbox" \
    "starred" \
    "--no-inbox --no-starred"

# Import Archived emails
import_mbox \
    "$MAIL_DIR/Archived.mbox" \
    "archived" \
    "--no-inbox"

# Final summary
echo ""
echo "=========================================="
echo -e "${GREEN}[ALL DONE]${NC}"
echo "=========================================="
echo ""
echo "All imports completed successfully!"
echo ""
echo "To verify in Gmail:"
echo "  1. Open Gmail"
echo "  2. Search for: import-$IMPORT_DATE"
echo "  3. You should see all imported emails with the following labels:"
echo "     - import-inbox-$IMPORT_DATE"
echo "     - import-sent-$IMPORT_DATE"
echo "     - import-starred-$IMPORT_DATE"
echo "     - import-archived-$IMPORT_DATE"
echo ""
