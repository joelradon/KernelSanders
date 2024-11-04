# ---------------------------------------------
# Script: ListAndPrintFileContents.ps1
# Description: Lists all files in the top-level directory and its subdirectories,
#              then prints the contents of each file while excluding README.MD.
# ---------------------------------------------

# ------------------------------
# 1. Define the Folder Path
# ------------------------------

# Use the current directory as the folder path.
$FolderPath = Get-Location

# ------------------------------
# 2. Define Allowed File Extensions (Optional)
# ------------------------------

# Specify which file types to process.
$AllowedExtensions = @("txt", "md", "csv", "log", "json", "xml", "ps1", "go", "py", "js", "html", "css")

# ------------------------------
# 3. Validate the Folder Path
# ------------------------------

if (-Not (Test-Path -Path $FolderPath)) {
    Write-Host "Error: The folder path '$FolderPath' does not exist." -ForegroundColor Red
    exit
}

# ------------------------------
# 4. Retrieve All Files
# ------------------------------

# Get all files within the directory and its subdirectories, excluding README.MD in the top-level directory.
try {
    $Files = Get-ChildItem -Path $FolderPath -Recurse -File | Where-Object {
        # Exclude README.MD from the top-level directory
        $_.FullName -notlike "$FolderPath\README.MD"
    }
} catch {
    Write-Host "Error retrieving files: $_" -ForegroundColor Red
    exit
}

# ------------------------------
# 5. Process Each File
# ------------------------------

# Store the output to be copied to the clipboard
$Output = ""

foreach ($File in $Files) {
    # Check if the file has an allowed extension (if filtering is enabled)
    if ($AllowedExtensions.Count -gt 0) {
        $fileExtension = $File.Extension.TrimStart('.').ToLower()
        if (-Not ($AllowedExtensions -contains $fileExtension)) {
            Write-Host "Skipping unsupported file type: $($File.FullName)" -ForegroundColor Yellow
            continue
        }
    }

    $Output += "----------------------------------------`n"
    $Output += "File: $($File.FullName)`n"
    $Output += "----------------------------------------`n"

    # Attempt to read and display the file content.
    try {
        $Content = Get-Content -Path $File.FullName -ErrorAction Stop -Raw
        $Output += $Content + "`n`n"
    } catch {
        # Append the error message to the output without ForegroundColor
        $Output += "Unable to read file: $($File.FullName). Error: $_`n`n"
    }
}

# ------------------------------
# 6. Copy Output to Clipboard
# ------------------------------

# Copy the output to the clipboard
Set-Clipboard -Value $Output

# ------------------------------
# 7. Completion Message
# ------------------------------

Write-Host "Completed processing all files in '$FolderPath'. Output copied to clipboard." -ForegroundColor Magenta
