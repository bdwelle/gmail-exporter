#!/usr/bin/env python3
import os
import re
import sys
import glob
import shutil
from pathlib import Path


def is_valid_mbox(file_path):
    """Check if file appears to be a valid mbox file"""
    try:
        with open(file_path, "r", encoding="utf-8", errors="ignore") as f:
            first_chunk = f.read(5000)

        # Check for common mbox patterns
        # Gmail mbox files may start with metadata, so check deeper in the file
        return "From " in first_chunk
    except Exception as e:
        print(f"Warning: Could not validate file: {e}", file=sys.stderr)
        return True


def split_mbox(
    mbox_path, output_dir, verbose=False, counter=None, progress_callback=None
):
    """Split an mbox file into individual .eml files

    Args:
        mbox_path: Path to .mbox file
        output_dir: Directory to save split .eml files
        verbose: Enable verbose output for debugging
        counter: Starting message number (for processing multiple files)
        progress_callback: Function to call with progress updates (file, count, total)

    Returns:
        tuple: (int messages saved, int new counter value)
    """

    # Create output directory if it doesn't exist
    os.makedirs(output_dir, exist_ok=True)

    filename = os.path.basename(mbox_path)
    file_size = os.path.getsize(mbox_path)
    file_size_mb = file_size / (1024 * 1024)

    if verbose:
        print(f"Reading mbox file: {mbox_path}", file=sys.stderr)
        print(f"Output directory: {output_dir}", file=sys.stderr)

    # Show file info
    print(f"Processing: {filename} ({file_size_mb:.1f} MB)", file=sys.stderr)

    with open(mbox_path, "r", encoding="utf-8", errors="ignore") as f:
        content = f.read()

    # Split by "From " lines (mbox format)
    messages = re.split(r"\nFrom ", content)
    total_in_file = len(messages)

    # Estimate total messages (skip first if empty)
    estimated_total = max(0, total_in_file - 1)

    print(f"  Estimated messages: {estimated_total}", file=sys.stderr)
    print("", file=sys.stderr)

    count = 0
    skipped = 0
    empty = 0

    # Use provided counter or start at 1
    if counter is None:
        counter = 1

    # Progress tracking
    last_progress = 0
    progress_interval = max(1, estimated_total // 20)  # Update 20 times max

    for i, msg in enumerate(messages):
        msg = msg.strip()
        if not msg:
            empty += 1
            if verbose:
                print(f"  Message {i}: Empty, skipping", file=sys.stderr)
            continue

        # Skip the first message if it's empty (from the split)
        if i == 0 and not msg.startswith("<!DOCTYPE"):
            if verbose:
                print(
                    f"  Message {i}: First message empty (no DOCTYPE), skipping",
                    file=sys.stderr,
                )
            skipped += 1
            continue

        # Re-add the "From " line (minus the first one)
        if i > 0:
            msg = "From " + msg

        # Remove the first "From " line content to get the actual email
        lines = msg.split("\n", 1)
        if len(lines) > 1 and lines[0].startswith("From "):
            msg = lines[1]

        # Skip empty messages after trimming
        if not msg.strip():
            empty += 1
            if verbose:
                print(f"  Message {i}: Empty after trimming, skipping", file=sys.stderr)
            continue

        # Save to file (use global counter)
        output_path = os.path.join(output_dir, f"msg.{counter:03d}.eml")
        try:
            with open(output_path, "w", encoding="utf-8") as out:
                out.write(msg)
            count += 1
            counter += 1

            # Show progress via callback
            if progress_callback:
                progress_callback(filename, count, estimated_total)

            # Show detailed progress (only in verbose mode)
            if verbose and count % 10 == 0:
                print(f"  Processed {count} messages...", file=sys.stderr)

        except Exception as e:
            print(f"Error writing message {i}: {e}", file=sys.stderr)
            continue

    print(f"\r", file=sys.stderr)  # Clear the progress line
    print(f"✓ Split {count} messages to {output_dir}/", file=sys.stderr)
    if skipped > 0:
        print(f"  Skipped {skipped} empty/invalid messages", file=sys.stderr)
    if empty > 0:
        print(f"  Found {empty} empty messages", file=sys.stderr)
    return count, counter


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <mbox_file|pattern> [output_dir] [options]")
        print("")
        print("Options:")
        print("  -v, --verbose    Enable verbose output")
        print("  -c, --clear      Clear output directory and reset counter")
        print("")
        print("Arguments:")
        print(
            '  mbox_file|pattern  Path to .mbox file or glob pattern (e.g., "mboxes/*.mbox")'
        )
        print(
            "  output_dir         Directory to save split .eml files (default: 2-split-to-import)"
        )
        print("")
        print("Examples:")
        print("  Split single file:  ./split-mbox-emails.py emails.mbox")
        print('  Split multiple files: ./split-mbox-emails.py "mboxes/*.mbox"')
        print(
            "  Split and append:    ./split-mbox-emails.py new.mbox 2-split-to-import -v"
        )
        print(
            "  Clear and restart:   ./split-mbox-emails.py *.mbox 2-split-to-import --clear"
        )
        sys.exit(1)

    # Parse arguments
    input_pattern = sys.argv[1]
    output_dir = "2-split-to-import"
    verbose = False
    clear_dir = False

    i = 2
    while i < len(sys.argv):
        arg = sys.argv[i]

        if arg in ["-v", "--verbose"]:
            verbose = True
            i += 1
        elif arg in ["-c", "--clear"]:
            clear_dir = True
            i += 1
        elif not arg.startswith("-"):
            output_dir = arg
            i += 1
        else:
            print(f"Error: Unknown argument: {arg}", file=sys.stderr)
            sys.exit(1)

    # Clear output directory if requested
    if clear_dir:
        if os.path.exists(output_dir):
            print(f"Clearing output directory: {output_dir}", file=sys.stderr)
            try:
                shutil.rmtree(output_dir)
                os.makedirs(output_dir, exist_ok=True)
            except Exception as e:
                print(f"Error clearing directory: {e}", file=sys.stderr)
                sys.exit(1)
        print("Counter reset to 1", file=sys.stderr)

    # Find matching files (supports glob patterns)
    if "*" in input_pattern or "?" in input_pattern or "[" in input_pattern:
        mbox_files = sorted(glob.glob(input_pattern))
        if not mbox_files:
            print(
                f"Error: No files found matching pattern: {input_pattern}",
                file=sys.stderr,
            )
            sys.exit(1)
    else:
        # Single file - check if it exists
        if not os.path.exists(input_pattern):
            print(f"Error: File not found: {input_pattern}")
            sys.exit(1)
        if not os.path.isfile(input_pattern):
            print(f"Error: {input_pattern} is not a file")
            sys.exit(1)
        mbox_files = [input_pattern]

    # Check if they are valid mbox files
    for mbox_file in mbox_files:
        if not is_valid_mbox(mbox_file):
            print(
                f"Warning: {mbox_file} does not appear to be a valid mbox file",
                file=sys.stderr,
            )
            print("Proceeding anyway...", file=sys.stderr)

    # Split all mbox files with a shared counter
    counter = 1
    total_messages = 0
    total_files = len(mbox_files)
    global_progress = {}

    print(f"Total files to process: {total_files}", file=sys.stderr)
    print("", file=sys.stderr)

    for idx, mbox_file in enumerate(mbox_files, 1):
        filename = os.path.basename(mbox_file)
        if verbose:
            print(f"\n{'=' * 50}", file=sys.stderr)
            print(f"Processing: {mbox_file}", file=sys.stderr)
        else:
            print(f"\n[{idx}/{total_files}] Processing: {filename}...", file=sys.stderr)

        # Progress tracking for this file
        with open(mbox_file, "r", encoding="utf-8", errors="ignore") as f:
            content = f.read()
            messages = re.split(r"\nFrom ", content)
            file_size = os.path.getsize(mbox_file)
            file_size_mb = file_size / (1024 * 1024)
            estimated_total = max(0, len(messages) - 1)

        print(f"  Size: {file_size_mb:.1f} MB", file=sys.stderr)
        print(f"  Estimated: {estimated_total} messages", file=sys.stderr)
        print("", file=sys.stderr)

        # Progress tracking for this file
        global_progress[filename] = {
            "count": 0,
            "total": estimated_total,
            "last_update": 0,
        }

        # Progress callback
        def show_progress(filename, count, total):
            # Only show progress in non-verbose mode
            if not verbose:
                pct = (count / total * 100) if total > 0 else 100
                bar_width = 30
                filled = int(bar_width * pct / 100)
                bar = "█" * filled + "░" * (bar_width - filled)
                print(
                    f"\r    [{bar}] {count:5d}/{total} ({pct:5.1f}%)",
                    end="",
                    file=sys.stderr,
                )
                sys.stderr.flush()

        count, counter = split_mbox(
            mbox_file, output_dir, verbose, counter, show_progress
        )
        total_messages += count

    print(
        f"\n✓ Total: {total_messages} messages split to {output_dir}/", file=sys.stderr
    )

    if total_messages == 0:
        print(f"Error: No valid emails found", file=sys.stderr)
        sys.exit(1)
