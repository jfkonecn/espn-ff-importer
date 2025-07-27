#!/bin/bash

# Exit on error, undefined variable, or pipe failure
set -euo pipefail

# Change to the directory where this script resides
cd "$(dirname "$0")"

# Check if arguments were provided or if --help was passed
if [ "$#" -eq 1 ] && [ "$1" = "--help" ]; then
    echo "Usage: $0 [data-directory] [output-directory]"
    echo ""
    echo "Arguments:"
    echo "  [data-directory]     Directory containing ESPN league JSON files (optional, defaults to 'data')"
    echo "  [output-directory]   Output directory for static website (optional, defaults to 'static')"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Use default data and static directories"
    echo "  $0 data                              # Use custom data directory"
    echo "  $0 data static                       # Use custom data and output directories"
    echo ""
    echo "The script will process all espn_league_*.json files in the data directory"
    echo "and generate a static website with an index page and individual season pages."
    exit 1
fi

# Get the data directory (first argument, optional)
DATA_DIR="data"
if [ "$#" -gt 0 ]; then
    DATA_DIR="$1"
fi

# Get the output directory (second argument, optional)
OUTPUT_DIR="static"
if [ "$#" -gt 1 ]; then
    OUTPUT_DIR="$2"
fi

# Check if data directory exists
if [ ! -d "$DATA_DIR" ]; then
    echo "Error: Data directory '$DATA_DIR' does not exist"
    exit 1
fi

# Check if there are any JSON files in the data directory
if ! ls "$DATA_DIR"/espn_league_*.json 1> /dev/null 2>&1; then
    echo "Error: No espn_league_*.json files found in '$DATA_DIR'"
    echo "Please ensure your JSON files are named like: espn_league_2024.json"
    exit 1
fi

# Build the command
CMD="go run src/*.go -data \"$DATA_DIR\" -output \"$OUTPUT_DIR\""

# Run the command
echo "Running: $CMD"
eval $CMD

echo ""
echo "Static website generated successfully!"
echo "Open $OUTPUT_DIR/index.html in your web browser to view the league website."
echo ""
echo "Generated files:"
ls -la "$OUTPUT_DIR"/*.html 2>/dev/null || echo "No HTML files found in $OUTPUT_DIR"
