# KernelSanders

**KernelSanders** is a robust Telegram bot designed to assist developers by allowing seamless interaction with their source code. Leveraging the power of OpenAI's language models, KernelSanders provides intelligent responses, manages user-uploaded source code files, and ensures data persistence and security through integration with AWS S3. The bot is optimized for both individual and group chats, offering functionalities tailored to enhance productivity and maintain privacy.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Configuration](#configuration)
  - [Running the Bot](#running-the-bot)
- [Commands](#commands)
  - [/start](#start)
  - [/help](#help)
  - [/upload](#upload)
  - [/mydata](#mydata)
  - [/security](#security)
  - [/project](#project)
  - [/my_source_code](#my_source_code)
- [Folder Structure](#folder-structure)
- [Usage](#usage)
  - [Uploading Source Code](#uploading-source-code)
  - [Interacting with the Bot](#interacting-with-the-bot)
- [Security Best Practices](#security-best-practices)
- [Contributing](#contributing)
- [License](#license)
- [Acknowledgements](#acknowledgements)

---

## Overview

KernelSanders is a Telegram bot that empowers developers by facilitating interactions with their source code files. Whether you're seeking intelligent code suggestions, need to manage your code snippets, or require context-aware responses from an AI assistant, KernelSanders has you covered. The bot ensures that your data is securely stored and automatically managed, providing a seamless and efficient coding companion.

## Features

- **Intelligent Responses:** Utilize OpenAI's GPT-4 model to receive context-aware and intelligent responses to your queries.
- **Source Code Management:** Upload `.txt` source code files that the bot can reference to provide more accurate assistance.
- **Persistent Storage:** All responses and uploaded files are securely stored in AWS S3 with automatic expiration after 4 hours.
- **Rate Limiting:** Prevents abuse by limiting the number of messages a user can send within a specific timeframe.
- **Web Response Links:** Generate short-lived web links for your responses, enhancing readability and navigation.
- **Group Chat Support:** Tailored functionalities for both individual and group chats, ensuring privacy and efficiency.

## Getting Started

Follow these instructions to set up and run KernelSanders on your local machine or server.

### Prerequisites

Before you begin, ensure you have met the following requirements:

- **Go:** [Install Go](https://golang.org/doc/install) (version 1.16 or higher recommended).
- **AWS Account:** Access to an AWS account with permissions to create and manage S3 buckets.
- **Telegram Bot Token:** Obtain a bot token by creating a new bot via [BotFather](https://t.me/BotFather) on Telegram.
- **AWS S3 Bucket:** Create an S3 bucket to store user data and responses.
- **Secret Manager (Recommended):** For securely managing environment variables and secrets.

### Installation

1. **Clone the Repository:**

   ```bash
   git clone https://github.com/joelradon/KernelSanders.git
   cd KernelSanders
   ```

2. **Install Dependencies:**

   Ensure all necessary Go packages are installed.

   ```bash
   go mod tidy
   ```

### Configuration

KernelSanders relies on several environment variables for configuration. It's highly recommended to use a secret manager (e.g., AWS Secrets Manager, HashiCorp Vault) to store these variables securely.

#### Required Environment Variables

- `TELEGRAM_TOKEN`: Your Telegram bot token obtained from BotFather.
- `OPENAI_KEY`: Your OpenAI API key.
- `OPENAI_ENDPOINT`: (Optional) Custom OpenAI API endpoint. Defaults to `https://api.openai.com/v1/chat/completions`.
- `BOT_USERNAME`: The username of your Telegram bot (without `@`).
- `AWS_ENDPOINT_URL_S3`: The endpoint URL for your AWS S3 service.
- `AWS_REGION`: The AWS region where your S3 bucket is located.
- `BUCKET_NAME`: The name of your AWS S3 bucket.
- `PORT`: (Optional) The port on which the server will run. Defaults to `8080`.
- `BASE_URL`: (Optional) The base URL for generating response and file links. Defaults to `http://localhost:8080`.
- `NO_LIMIT_USERS`: (Optional) Comma-separated list of Telegram user IDs exempt from rate limiting.

#### Setting Environment Variables

You can set environment variables in your shell or use a `.env` file with a secret manager. Here's an example using a `.env` file:

```dotenv
TELEGRAM_TOKEN=your_telegram_bot_token
OPENAI_KEY=your_openai_api_key
OPENAI_ENDPOINT=https://api.openai.com/v1/chat/completions
BOT_USERNAME=KernelSandersBot
AWS_ENDPOINT_URL_S3=https://s3.your-region.amazonaws.com
AWS_REGION=your-aws-region
BUCKET_NAME=your-s3-bucket-name
PORT=8080
BASE_URL=https://your-domain.com
NO_LIMIT_USERS=123456789,987654321
```

> **Security Note:** Never commit your `.env` file or any sensitive information to version control. Always use secret managers or environment variable configurations provided by your deployment platform.

### Running the Bot

1. **Build the Application:**

   ```bash
   go build -o kernelsanders cmd/main.go
   ```

2. **Run the Application:**

   ```bash
   ./kernelsanders
   ```

   The server will start on the specified port (default `8080`). Ensure that this port is accessible and properly configured in your Telegram bot webhook settings.

3. **Set Telegram Webhook:**

   To receive updates from Telegram, set your bot's webhook to point to your server's URL.

   ```bash
   curl -F "url=https://your-domain.com/" https://api.telegram.org/bot<YOUR_TELEGRAM_TOKEN>/setWebhook
   ```

   Replace `https://your-domain.com/` with your actual server URL and `<YOUR_TELEGRAM_TOKEN>` with your bot token.

## Commands

KernelSanders offers a variety of commands to enhance your interaction and manage your data effectively. Here's a comprehensive list of available commands accessible via the `/help` command.

### /start

**Description:** Initializes interaction with the bot.

**Usage:**

Simply send `/start` to the bot to receive a welcome message and basic instructions.

**Example:**

```
/start
```

**Response:**

```
🎉 *Welcome to Kernel Sanders Bot!*

You can ask me questions about your application or upload your source code files for more context.
```

### /help

**Description:** Displays a help menu with all available commands and their descriptions.

**Usage:**

Send `/help` to the bot to view the list of supported commands and their functionalities.

**Example:**

```
/help
```

**Response:**

```
🔍 *Help Menu:*

*Commands:*
/start - Start interacting with the bot
/help - Show this help message
/upload - Upload your source code file (only .txt files are supported)
/mydata - View your uploaded files and web responses
/security - Learn about the bot's security measures
/project - Learn about the KernelSanders project and how to contribute
/my_source_code - Get scripts to prepare your source code for upload

*File Uploads:*
In group chats, upload .txt files by tagging me in the caption using @KernelSandersBot. In 1-on-1 chats, simply send the .txt file without tagging.

These files will be stored for *4 hours* only. Uploading a new file will overwrite the existing one and reset the storage time.

*Short-Lived Web Responses:*
The bot provides short-lived web response links for easier reading and navigation of your code outputs. Please save any outputs or files you wish to use for long-term purposes, as the web responses will expire after the specified duration.

🛡️ *Security:* Only .txt files are accepted to prevent potential security risks associated with other file types.
```

### /upload

**Description:** Allows users to upload their source code files. Only `.txt` files are supported to ensure security.

**Usage:**

- **Private Chats:** Send a `.txt` file directly to the bot without tagging.
- **Group Chats:** Upload a `.txt` file and tag the bot in the caption using `@KernelSandersBot`.

**Example:**

In a private chat:

1. Click on the attachment icon.
2. Select and send your `.txt` source code file.

In a group chat:

1. Attach a `.txt` file.
2. In the caption, include `@KernelSandersBot` to ensure the bot processes the upload.

**Response:**

```
✅ *File Uploaded Successfully*

Your source code has been uploaded and will be stored until:

• *Upload Time:* UTC: Mon, 05 Oct 2024 14:00:00 UTC | EDT: Mon, 05 Oct 2024 10:00:00 EDT
• *Deletion Time:* UTC: Mon, 05 Oct 2024 18:00:00 UTC | EDT: Mon, 05 Oct 2024 14:00:00 EDT

Please save any work or prompts that may be useful in the future.
```

### /mydata

**Description:** Retrieves a list of all your uploaded files and generated web responses.

**Usage:**

Send `/mydata` to the bot to view your current uploads and responses.

**Example:**

```
/mydata
```

**Response:**

```
🔍 *Your Data:*

*Uploaded Files:*
| File Name                        | Uploaded At (UTC)          | Uploaded At (EDT)          | Deletion Time (UTC)         | Deletion Time (EDT)         |
|----------------------------------|----------------------------|----------------------------|-----------------------------|-----------------------------|
| https://your-domain.com/files/source_code.txt | Mon, 05 Oct 2024 14:00:00 UTC | Mon, 05 Oct 2024 10:00:00 EDT | Mon, 05 Oct 2024 18:00:00 UTC | Mon, 05 Oct 2024 14:00:00 EDT |

*Web Responses:*
| Response ID                      | Created At (UTC)           | Created At (EDT)           | Deletion Time (UTC)         | Deletion Time (EDT)         |
|----------------------------------|----------------------------|----------------------------|-----------------------------|-----------------------------|
| abc123-def456-ghi789              | Mon, 05 Oct 2024 14:05:00 UTC | Mon, 05 Oct 2024 10:05:00 EDT | Mon, 05 Oct 2024 18:05:00 UTC | Mon, 05 Oct 2024 14:05:00 EDT |
```

### /security

**Description:** Provides information about the bot's security measures and data handling practices.

**Usage:**

Send `/security` to the bot to learn about how your data is protected.

**Example:**

```
/security
```

**Response:**

```
🔐 *Security Information:*

Your data and responses are handled with the utmost security. Uploaded files are stored securely in S3 with strict access controls and are automatically deleted after 4 hours. All interactions are logged for auditing purposes.

The project's source code is open-source, allowing for community review and contributions. You can view the code on GitHub here: [KernelSanders GitHub](https://github.com/joelradon/KernelSanders).

Feel free to review the code and contribute to its development!
```

### /project

**Description:** Shares details about the KernelSanders project, its purpose, and how you can contribute.

**Usage:**

Send `/project` to the bot to learn more about KernelSanders and ways to get involved.

**Example:**

```
/project
```

**Response:**

```
🚀 *KernelSanders Project:*

The KernelSanders bot is an open-source project designed to assist you with your coding needs. Contributions are welcome! You can view the source code and contribute on GitHub: <a href="https://github.com/joelradon/KernelSanders">KernelSanders GitHub</a>.

If you find this tool useful, consider buying me a coffee: <a href="https://paypal.me/joelradon">Buy me a Coffee</a>. Your support is greatly appreciated! ☕😊
```

### /my_source_code

**Description:** Provides scripts to prepare your source code for upload, ensuring only relevant files are processed.

**Usage:**

Send `/my_source_code` to the bot to receive download links for preparation scripts.

**Example:**

```
/my_source_code
```

**Response:**

```
📁 *Prepare Your Source Code for Upload:*

Use the following scripts to quickly copy and prepare your source code in a directory tree for upload. These scripts exclude README files and only process specified file types.

*PowerShell Script:* <a href="https://s3.amazonaws.com/your-bucket/powershell_prepare_source.ps1">Download PowerShell Script</a>

*Bash Script:* <a href="https://s3.amazonaws.com/your-bucket/bash_prepare_source.sh">Download Bash Script</a>

These scripts will generate a structured output of your code files, making it easier to upload and manage your projects.
```

## Folder Structure

Understanding the project's directory structure is crucial for navigation, development, and contribution. Here's a breakdown of each folder and its role within the KernelSanders application.

```
KernelSanders/
├── cmd/
│   └── main.go
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   └── response_store.go
│   ├── api/
│   │   └── api_requests.go
│   ├── cache/
│   │   └── cache.go
│   ├── conversation/
│   │   └── conversation_cache.go
│   ├── handlers/
│   │   └── handlers.go
│   ├── s3client/
│   │   └── s3client.go
│   ├── telegram/
│   │   └── telegram_handler.go
│   ├── types/
│   │   └── types.go
│   ├── usage/
│   │   └── usage_cache.go
│   └── utils/
│       └── utils.go
├── utility_scripts/
│   ├── copy_source_code.ps1
│   └── README.MD
├── go.mod
├── go.sum
└── README.MD
```

### `cmd/`

- **main.go:** The entry point of the application. Initializes the app, sets up HTTP handlers for Telegram updates and web requests, and starts the server.

### `internal/`

Contains the core components of the KernelSanders application.

#### `app/`

- **app.go:** Initializes and manages the main application, including configurations, dependencies, and core functionalities like message processing, rate limiting, and logging.
- **response_store.go:** Manages storage and retrieval of user responses, ensuring persistence in AWS S3 and handling expiration of data.

#### `api/`

- **api_requests.go:** Handles interactions with external APIs, specifically the OpenAI API. Manages sending requests and parsing responses.

#### `cache/`

- **cache.go:** Implements a thread-safe in-memory cache for storing temporary data.

#### `conversation/`

- **conversation_cache.go:** Manages conversation contexts for users, ensuring context-aware interactions and handling expiration of inactive sessions.

#### `handlers/`

- **handlers.go:** Defines the `MessageProcessor` interface, outlining the methods required for processing messages, handling commands, sending responses, and managing user data.

#### `s3client/`

- **s3client.go:** Implements the S3 client interface for interacting with AWS S3. Handles operations like getting, putting, listing, and deleting objects in the S3 bucket.

#### `telegram/`

- **telegram_handler.go:** Handles incoming Telegram messages, including text and document uploads. Manages command parsing, message processing, and file handling.

#### `types/`

- **types.go:** Defines various data structures and types used across the application, including Telegram update structures, user data representations, and OpenAI API payloads.

#### `usage/`

- **usage_cache.go:** Implements rate limiting by tracking user message usage, ensuring fair usage and preventing abuse.

#### `utils/`

- **utils.go:** Provides utility functions for text summarization, keyword extraction, and time formatting.

### `utility_scripts/`

Contains scripts to assist users in preparing their source code for upload.

- **copy_source_code.ps1:** A PowerShell script that lists all files in the current directory and its subdirectories, excluding `README.MD`, and copies their contents to the clipboard.
- **README.MD:** Documentation for the utility scripts, explaining their purpose and usage.

## Usage

Once you've set up and run the KernelSanders bot, you can interact with it through Telegram. Here's how to make the most out of its functionalities.

### Uploading Source Code

KernelSanders allows you to upload your source code files to provide context for more accurate and relevant responses.

**Supported File Type:** Only `.txt` files are accepted to ensure security and simplicity.

**Uploading in Private Chats:**

1. Open a private chat with the KernelSanders bot.
2. Click on the attachment icon (📎).
3. Select and send your `.txt` source code file directly.

**Uploading in Group Chats:**

1. In a group chat, attach your `.txt` source code file.
2. In the caption, tag the bot using `@KernelSandersBot` to ensure it processes the upload.

**Note:** Uploaded files are stored for **4 hours**. Uploading a new file will overwrite the existing one and reset the storage time.

### Interacting with the Bot

You can ask KernelSanders various questions related to your application, get code suggestions, or request explanations. The bot uses the uploaded source code to provide context-aware responses.

**Example Interaction:**

1. **User:** "How can I optimize my sorting algorithm?"
2. **KernelSanders:** Provides suggestions based on the uploaded source code.

**Short-Lived Web Responses:**

For better readability and navigation, KernelSanders generates web links for your responses. These links are temporary and will expire after the specified duration.

**Example:**

```
Here is your optimized sorting algorithm: [View Formatted Response](https://your-domain.com/abc123-def456-ghi789)
```

## Security Best Practices

KernelSanders prioritizes the security and privacy of your data. Here are the best practices implemented and recommended for users:

1. **Secure Storage:**
   - All uploaded files and responses are stored in AWS S3 with strict access controls.
   - Files are automatically deleted after 4 hours to minimize data exposure.

2. **Environment Variables Management:**
   - **Recommendation:** Use a secret manager (e.g., AWS Secrets Manager, HashiCorp Vault) to store environment variables and sensitive information like API keys and tokens.
   - Avoid hardcoding secrets or committing them to version control systems.

3. **File Validation:**
   - Only `.txt` files are accepted to prevent potential security risks associated with executable or binary files.
   - Ensures that uploaded content is plain text, reducing the risk of code injection or malware.

4. **Rate Limiting:**
   - Implements rate limiting to prevent abuse and ensure fair usage among users.
   - Exemptions are available for trusted users to maintain flexibility.

5. **Logging and Auditing:**
   - All interactions are logged and stored securely for auditing purposes.
   - Helps in monitoring usage patterns and identifying potential security threats.

6. **Open-Source Transparency:**
   - The project's source code is open for community review, fostering transparency and collaborative security enhancements.

7. **User Data Privacy:**
   - Personal data and uploaded files are handled with utmost confidentiality.
   - Encourages users not to upload sensitive information despite the secure handling mechanisms.

**Additional Recommendations for Users:**

- **Avoid Uploading Sensitive Information:** While the bot manages data securely, always refrain from uploading sensitive or confidential code.
- **Regularly Update Dependencies:** Ensure that all dependencies and packages are up-to-date to benefit from security patches.
- **Monitor Access Logs:** Regularly review access logs for any unusual activity or unauthorized access attempts.

## Contributing

Contributions are welcome! Whether you're reporting a bug, suggesting a feature, or contributing code, your support helps improve KernelSanders.

### How to Contribute

1. **Fork the Repository:**

   Click the "Fork" button at the top right of the repository page to create your own copy.

2. **Clone the Repository:**

   ```bash
   git clone https://github.com/your-username/KernelSanders.git
   cd KernelSanders
   ```

3. **Create a New Branch:**

   ```bash
   git checkout -b feature/YourFeatureName
   ```

4. **Make Your Changes:**

   Implement your feature or bug fix.

5. **Commit Your Changes:**

   ```bash
   git commit -m "Add Your Feature Description"
   ```

6. **Push to Your Fork:**

   ```bash
   git push origin feature/YourFeatureName
   ```

7. **Submit a Pull Request:**

   Navigate to the original repository and submit a pull request detailing your changes.

### Code of Conduct

Please adhere to the [Code of Conduct](https://github.com/joelradon/KernelSanders/blob/main/CODE_OF_CONDUCT.md) in all interactions.

## License

Distributed under the MIT License. See `LICENSE` for more information.

## Acknowledgements

- [OpenAI](https://openai.com/) for providing powerful language models.
- [AWS S3](https://aws.amazon.com/s3/) for reliable and scalable storage solutions.
- [Telegram Bot API](https://core.telegram.org/bots/api) for enabling rich bot functionalities.
- [Go Programming Language](https://golang.org/) for its simplicity and performance.
- [Various Open Source Contributors](https://github.com/joelradon/KernelSanders/graphs/contributors) for their valuable contributions.

---

## Utility Scripts

KernelSanders includes utility scripts to help you prepare and manage your source code files efficiently.

### PowerShell Script: `copy_source_code.ps1`

**Description:** Lists all files in the current directory and its subdirectories, excluding `README.MD`, and copies their contents to the clipboard. Only processes specified file types to ensure relevance and security.

**Usage:**

1. Open PowerShell.
2. Navigate to the directory containing your source code.
3. Run the script:

   ```powershell
   ./copy_source_code.ps1
   ```

4. The script will process the files and copy the output to your clipboard.

**Note:** Ensure that the script has the necessary execution permissions. You can set the execution policy using:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

> **Security Reminder:** Always review and understand scripts before executing them to prevent potential security risks.

### README for Utility Scripts: `utility_scripts/README.MD`

Provides detailed information about the utility scripts, their functionalities, and usage instructions.

---

## Support

If you encounter any issues or have questions, feel free to open an issue on the [GitHub repository](https://github.com/joelradon/KernelSanders/issues) or reach out to the maintainer directly.

---

**Enjoy using KernelSanders! Your intelligent coding companion. 🚀**