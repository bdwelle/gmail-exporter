# Gmail Import Batch Script

This script automates importing all main Gmail folders from Google Takeout.

## What it does

Imports these 4 mbox files in sequence:
1. **Inbox.mbox** - Received emails (with label `import-inbox-DATE`)
2. **Sent.mbox** - Sent emails (with label `import-sent-DATE`)
3. **Starred.mbox** - Starred emails (with label `import-starred-DATE`)
4. **Archived.mbox** - Archived emails (with label `import-archived-DATE`)

## Why these 4 files?

Google Takeout creates multiple mbox files:
- **Inbox.mbox** - All received emails (~8.1G)
- **Sent.mbox** - All sent emails (~16G)
- **Starred.mbox** - All starred emails (~2.5G)
- **Archived.mbox** - All archived emails (~19G)
- Category files (Personal, Social, etc.) - **Subsets of Inbox**

**Category files are skipped** because they contain duplicate emails from Inbox.mbox.
If you imported both, you'd have duplicate emails in Gmail.

## Prerequisites

1. Google Takeout extracted to `/Volumes/Samsung 2T/GoogleTakeout_2025-11/Mail/`
2. `split-mbox-emails-streaming.py` script in current directory (streaming version for large files)
3. `gmail-exporter` binary built and in current directory
4. Gmail authentication configured (run `./gmail-exporter auth` first)
5. Import labels created in Gmail: `import-inbox-YYYY-MM-DD`, etc.

## Usage

### Step 1: Update configuration (optional)

Edit `import-all.sh` to change:
- `MAIL_DIR` - Path to your Google Takeout Mail directory
- `IMPORT_DATE` - Date stamp for import labels (auto-generated as YYYY-MM-DD)

### Step 2: Run the script

```bash
./import-all.sh
```

### Step 3: Verify in Gmail

1. Open Gmail
2. Search for: `import-YYYY-MM-DD` (e.g., import-2026-02-05)
3. You should see all imported emails with labels:
   - `import-inbox-YYYY-MM-DD`
   - `import-sent-YYYY-MM-DD`
   - `import-starred-YYYY-MM-DD`
   - `import-archived-YYYY-MM-DD`

## What happens during import

### Inbox.mbox
- Split to individual .eml files
- Import with `--skip-duplicates --skip-categories --no-inbox --no-starred --no-important`
- Add label `import-inbox-YYYY-MM-DD` (auto-generated date)
- Result: Imported to Gmail without Inbox/Starred/Important flags

### Sent.mbox
- Split to individual .eml files
- Import with `--skip-duplicates --skip-categories --no-inbox`
- Add label `import-sent-YYYY-MM-DD` (auto-generated date)
- Result: Imported to Gmail with SENT label (appears in Sent folder)

### Starred.mbox
- Split to individual .eml files
- Import with `--skip-duplicates --skip-categories --no-inbox --no-starred`
- Add label `import-starred-YYYY-MM-DD` (auto-generated date)
- Result: Imported to Gmail with Starred label

### Archived.mbox
- Split to individual .eml files
- Import with `--skip-duplicates --skip-categories --no-inbox`
- Add label `import-archived-YYYY-MM-DD` (auto-generated date)
- Result: Imported to Gmail

## Flags explained

| Flag | Purpose | When used |
|-------|----------|-----------|
| `--skip-duplicates` | Skip emails already in Gmail | Always |
| `--skip-categories` | Skip Gmail category labels | Always (categories are system-managed) |
| `--no-inbox` | Don't add Inbox label | All except Inbox |
| `--no-starred` | Don't add Starred label | Inbox, Starred |
| `--no-important` | Don't add Important label | Inbox only |

## Troubleshooting

### Script fails with "File not found"
- Update `MAIL_DIR` in script to correct path
- Check that Google Takeout is extracted

### Import fails with authentication error
- Run `./gmail-exporter auth` first
- Check that credentials file exists

### Duplicate emails appear in Gmail
- Make sure `--skip-duplicates` is used (it is by default)
- Check that Message-ID headers are consistent

### Some emails missing in Gmail
- Check logs for errors
- Verify `import-DATE` labels are being applied
- Use `--limit 10` to test with a small batch first

## Importing specific files only

If you want to import just one folder:

```bash
# Import only Inbox
./split-mbox-emails.py '/Volumes/Samsung 2T/GoogleTakeout_2025-11/Mail/Inbox.mbox' --clear
./gmail-exporter import --skip-duplicates --no-inbox --no-starred --no-important --skip-categories \
  --input-dir 2-split-to-import --add-label="import-inbox-$(date +%Y-%m-%d)"

# Import only Sent
./split-mbox-emails.py '/Volumes/Samsung 2T/GoogleTakeout_2025-11/Mail/Sent.mbox' --clear
./gmail-exporter import --skip-duplicates --skip-categories \
  --input-dir 2-split-to-import --add-label="import-sent-$(date +%Y-%m-%d)"
```
