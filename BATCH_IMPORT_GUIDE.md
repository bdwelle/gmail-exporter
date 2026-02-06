# Gmail Batch Import - Quick Start Guide

## What you need:

1. ✅ Google Takeout extracted to `/Volumes/Samsung 2T/GoogleTakeout_2025-11/Mail/`
2. ✅ Labels created in Gmail: `import-inbox-YYYY-MM-DD`, `import-sent-YYYY-MM-DD`, etc.
3. ✅ Gmail authenticated: `./gmail-exporter auth`
4. ✅ Files ready: `import-all.sh`, `split-mbox-emails-streaming.py`, `gmail-exporter`

## Quick Start:

```bash
# Run the batch import script
./import-all.sh
```

## What happens:

1. 📧 **Inbox.mbox** → Imported with label `import-inbox-$(date +%Y-%m-%d)`
2. 📤 **Sent.mbox** → Imported with label `import-sent-$(date +%Y-%m-%d)`
3. ⭐ **Starred.mbox** → Imported with label `import-starred-$(date +%Y-%m-%d)`
4. 📦 **Archived.mbox** → Imported with label `import-archived-$(date +%Y-%m-%d)`

## After import:

1. 📖 Open Gmail
2. 🔍 Search for: `import-2026-02-05` (or whatever today's date is)
3. ✅ Verify all imported emails appear with their labels
4. 🗑️ Delete the `import-YYYY-MM-DD` labels when done (optional)

## Need to import again tomorrow?

Just run `./import-all.sh` again - it will use tomorrow's date automatically!

## Questions?

- See `IMPORT_ALL_README.md` for detailed documentation
- See `IMPORT_FLAGS.txt` for flag reference
