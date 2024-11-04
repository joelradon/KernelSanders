#!/bin/bash

# ------------------------------
# 1. Define the Folder Path
# ------------------------------

# Use the current directory as the folder path.
FolderPath=$(pwd)

# ------------------------------
# 2. Define Allowed File Extensions (Optional)
# ------------------------------

# Specify which file types to process.
AllowedExtensions=("txt" "md" "csv" "log" "json" "xml" "ps1" "go" "py" "js" "html" "css")

# ------------------------------
# 3. Validate the Folder Path
# ------------------------------

if [ ! -d "$FolderPath" ]; then
    echo "Error: The folder path '$FolderPath' does not exist."
    exit 1
fi

# ------------------------------
# 4. Retrieve All Files
# ------------------------------

# Store the output to be copied to the clipboard
Output=""

# Find and process files, excluding README.md in the top-level directory
while IFS= read -r -d '' File; do
    # Check if the file is README.md in the top-level directory
    if [[ "$File" == "$FolderPath/README.md" ]]; then
        continue
    fi

    # Get the file extension
    fileExtension="${File##*.}"

    # Check if the file has an allowed extension (if filtering is enabled)
    if [[ ${#AllowedExtensions[@]} -gt 0 ]] && ! [[ " ${AllowedExtensions[@]} " =~ " $fileExtension " ]]; then
        echo "Skipping unsupported file type: $File"
        continue
    fi

    Output+="----------------------------------------\n"
    Output+="File: $File\n"
    Output+="----------------------------------------\n"

    # Attempt to read and display the file content.
    if ! Content=$(< "$File" 2>/dev/null); then
        Output+="Unable to read file: $File. Error: $?\n\n"
    else
        Output+="$Content\n\n"
    fi
done < <(find "$FolderPath" -type f -print0)

# ------------------------------
# 6. Copy Output to Clipboard
# ------------------------------

# Copy the output to the clipboard (using xclip or pbcopy depending on your OS)
if command -v xclip &> /dev/null; then
    echo -n "$Output" | xclip -selection clipboard
elif command -v pbcopy &> /dev/null; then
    echo -n "$Output" | pbcopy
else
    echo "Warning: Clipboard utility not found. Output will not be copied to clipboard."
fi

# ------------------------------
# 7. Completion Message
# ------------------------------

echo -e "Completed processing all files in '$FolderPath'. Output copied to clipboard."