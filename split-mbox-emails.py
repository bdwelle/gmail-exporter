#!/usr/bin/env python3
import os
import re
import sys
from pathlib import Path


def is_valid_mbox(file_path):
    """Check if file appears to be a valid mbox file"""
    try:
        with open(file_path, "r", encoding="utf-8", errors="ignore") as f:
            first_chunk = f.read(1000)

        # Check for common mbox patterns
        return "From " in first_chunk and ("\nFrom " in first_chunk[:2000])
    except Exception as e:
        print(f"Warning: Could not validate file: {e}", file=sys.stderr)
        return True


def split_mbox(mbox_path, output_dir, verbose=False):
    """Split an mbox file into individual .eml files

    Args:
        mbox_path: Path to the .mbox file
        output_dir: Directory to save split .eml files
        verbose: Enable verbose output for debugging

    Returns:
        int: Number of messages split and saved
    """

    # Create output directory if it doesn't exist
    os.makedirs(output_dir, exist_ok=True)

    if verbose:
        print(f"Reading mbox file: {mbox_path}", file=sys.stderr)
        print(f"Output directory: {output_dir}", file=sys.stderr)

    with open(mbox_path, "r", encoding="utf-8", errors="ignore") as f:
        content = f.read()

    # Split by "From " lines (mbox format)
    messages = re.split(r"\nFrom ", content)

    count = 0
    skipped = 0
    empty = 0

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

        # Save to file
        output_path = os.path.join(output_dir, f"msg.{i:03d}.eml")
        try:
            with open(output_path, "w", encoding="utf-8") as out:
                out.write(msg)
            count += 1
            if verbose and count % 10 == 0:
                print(f"  Processed {count} messages...", file=sys.stderr)
        except Exception as e:
            print(f"Error writing message {i}: {e}", file=sys.stderr)
            continue

    print(f"✓ Split {count} messages to {output_dir}/")
    if skipped > 0:
        print(f"  Skipped {skipped} empty/invalid messages", file=sys.stderr)
    if empty > 0:
        print(f"  Found {empty} empty messages", file=sys.stderr)
    return count


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <mbox_file> [output_dir] [options]")
        print("")
        print("Options:")
        print("  -v, --verbose    Enable verbose output")
        print("")
        print("Arguments:")
        print("  mbox_file     Path to the .mbox file to split")
        print(
            "  output_dir     Directory to save split .eml files (default: 2-split-to-import)"
        )
        sys.exit(1)

    # Parse arguments
    mbox_path = sys.argv[1]
    output_dir = "2-split-to-import"
    verbose = False

    i = 2
    while i < len(sys.argv):
        arg = sys.argv[i]

        if arg in ["-v", "--verbose"]:
            verbose = True
            i += 1
        elif not arg.startswith("-"):
            output_dir = arg
            i += 1
        else:
            print(f"Error: Unknown argument: {arg}", file=sys.stderr)
            sys.exit(1)

    # Validate input file
    if not os.path.exists(mbox_path):
        print(f"Error: File not found: {mbox_path}")
        sys.exit(1)

    if not os.path.isfile(mbox_path):
        print(f"Error: {mbox_path} is not a file")
        sys.exit(1)

    # Check if it's a valid mbox file
    if not is_valid_mbox(mbox_path):
        print(
            f"Warning: {mbox_path} does not appear to be a valid mbox file",
            file=sys.stderr,
        )
        print("Proceeding anyway...", file=sys.stderr)

    # Split the mbox file
    count = split_mbox(mbox_path, output_dir, verbose)

    if count == 0:
        print(f"Error: No valid emails found in {mbox_path}", file=sys.stderr)
        sys.exit(1)
