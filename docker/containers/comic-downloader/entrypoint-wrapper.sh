#!/bin/sh
# This wrapper forces the output directory to always be /downloads.
# It rebuilds the argument list by skipping any user-supplied --output-dir (or -o)
# and then appending --output-dir /downloads.

found_url=0
new_args=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-dir|-o)
      # Skip this flag and its accompanying value.
      shift 2
      ;;
    *)
      new_args="$new_args $1"
      found_url=1
      shift
      ;;
  esac
done

# If no non-flag argument (likely the URL) is provided, run idle mode.
if [ $found_url -eq 0 ]; then
  echo "No URL provided. Starting idle mode..."
  exec tail -f /dev/null
fi

# Rebuild the positional parameters.
# This simple approach works if none of the arguments include spaces.
set -- $new_args --output-dir /downloads

echo "Running _comic-downloader with arguments: $@"

# Execute the underlying binary.
exec /usr/bin/_comic-downloader "$@"
