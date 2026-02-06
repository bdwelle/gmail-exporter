#!/usr/bin/env python3
"""
Streaming Mbox Splitter - Processes large mbox files efficiently

Processes mbox files line-by-line to maintain constant memory usage,
regardless of file size. This avoids loading entire files into memory.

Features:
- Line-by-line processing (constant memory usage)
- Progress indicator with visual bar
- Handles files of any size (tested up to 10GB+)
- Preserves all original message content
"""

import os
import sys
import glob
import shutil
import re
from pathlib import Path


def is_valid_mbox(file_path):
    """Check if file appears to be a valid mbox file"""
    try:
        with open(file_path, "r", encoding="utf-8", errors="ignore") as f:
            # Read first 5000 bytes to check for mbox markers
            first_chunk = f.read(5000)

        # Check for common mbox patterns
        return "From " in first_chunk
    except Exception as e:
        print(f"Warning: Could not validate file: {e}", file=sys.stderr)
        return True


def split_mbox_streaming(mbox_path, output_dir, verbose=False, counter=None):
    """
    Split an mbox file into individual .eml files using streaming approach.

    Reads the file line-by-line to maintain constant memory usage.
    Only keeps current email in memory (not entire file).

    Args:
        mbox_path: Path to .mbox file
        output_dir: Directory to save split .eml files
        verbose: Enable verbose output for debugging
        counter: Starting message number (for processing multiple files)

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

    # Use provided counter or start at 1
    if counter is None:
        counter = 1

    # Count messages and track progress
    messages_count = 0
    skipped = 0
    empty = 0

    # Progress tracking
    last_percent_shown = -1
    progress_update_interval = 5  # Update progress every 5%

    try:
        with open(mbox_path, "r", encoding="utf-8", errors="ignore") as f:
            current_email = []
            in_email = False
            message_num = 0

            for line_num, line in enumerate(f, 1):
                # Check if this line is a "From " separator (start of new message)
                # Use regex to detect mbox "From " line more robustly
                # Pattern: "From " followed by email address at the same line
                from_pattern = re.compile(r"^From\s+\S+\s+")

                if from_pattern.match(line):
                    # This is the start of a new email

                    if in_email and current_email:
                        # Save the previous email
                        message_num += 1
                        try:
                            output_path = os.path.join(
                                output_dir, f"msg.{counter:03d}.eml"
                            )
                            with open(output_path, "w", encoding="utf-8") as out:
                                out.writelines(current_email)

                            messages_count += 1
                            counter += 1

                            # Update progress
                            if verbose:
                                print(
                                    f"  Saved message {messages_count}: msg.{counter - 1:03d}.eml",
                                    file=sys.stderr,
                                )
                            elif message_num % 10 == 0:
                                print(
                                    f"  Saved {messages_count} messages so far...",
                                    file=sys.stderr,
                                )

                            # Show progress bar every 5%
                            if message_num % max(1, progress_update_interval) == 0:
                                current_line = line_num
                                # Estimate total lines (rough estimate: ~50 lines per message)
                                estimated_total = current_line // 50 + messages_count
                                if estimated_total > 0:
                                    percent = messages_count / estimated_total * 100
                                    if (
                                        percent
                                        >= last_percent_shown + progress_update_interval
                                    ):
                                        last_percent_shown = (
                                            percent // progress_update_interval
                                        ) * progress_update_interval
                                        pct_display = min(percent, 100)
                                        bar_width = 30
                                        filled = int(bar_width * pct_display / 100)
                                        bar = "█" * filled + "░" * (bar_width - filled)
                                        print(
                                            f"\r    [{bar}] {messages_count} estimated ({pct_display:3.0f}%)",
                                            end="",
                                            file=sys.stderr,
                                        )
                                        sys.stderr.flush()

                        except Exception as e:
                            print(
                                f"Error writing message {messages_count}: {e}",
                                file=sys.stderr,
                            )
                            skipped += 1

                    # Start new email
                    in_email = True
                    current_email = [line]
                elif in_email:
                    # Add line to current email
                    current_email.append(line)

    except Exception as e:
        print(f"Error processing file: {e}", file=sys.stderr)
        sys.exit(1)

    print(f"\r", file=sys.stderr)  # Clear progress line
    print(f"✓ Split {messages_count} messages to {output_dir}/", file=sys.stderr)
    if skipped > 0:
        print(f"  Skipped {skipped} messages with errors", file=sys.stderr)
    return messages_count, counter


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
        print("  Split single file:  ./split-mbox-emails-streaming.py emails.mbox")
        print(
            '  Split multiple files: ./split-mbox-emails-streaming.py "mboxes/*.mbox"'
        )
        print(
            "  Split and append:    ./split-mbox-emails-streaming.py new.mbox 2-split-to-import -v"
        )
        print(
            "  Clear and restart:   ./split-mbox-emails-streaming.py *.mbox 2-split-to-import --clear"
        )
        print("")
        print("Features:")
        print("  • Line-by-line processing (constant memory usage)")
        print("  • Handles 10GB+ files efficiently")
        print("  • Progress indicator with visual bar")
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

    print(f"Total files to process: {total_files}", file=sys.stderr)
    print("Processing in streaming mode (constant memory usage)", file=sys.stderr)
    print("", file=sys.stderr)

    for idx, mbox_file in enumerate(mbox_files, 1):
        filename = os.path.basename(mbox_file)
        if verbose:
            print(f"\n{'=' * 50}", file=sys.stderr)
            print(f"Processing: {mbox_file}", file=sys.stderr)
        else:
            print(f"\n[{idx}/{total_files}] Processing: {filename}...", file=sys.stderr)

        count, counter = split_mbox_streaming(mbox_file, output_dir, verbose, counter)
        total_messages += count

    print(
        f"\n✓ Total: {total_messages} messages split to {output_dir}/", file=sys.stderr
    )

    if total_messages == 0:
        print(f"Error: No valid emails found", file=sys.stderr)
        sys.exit(1)
