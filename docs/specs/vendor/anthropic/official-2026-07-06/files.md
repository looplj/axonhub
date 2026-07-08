[Claude Platform Docs](/docs/en/home)

Files API

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Loading...

Messages/Working with files

# Files API

The Files API lets you upload and manage files to use with the Claude API without re-uploading content with each request. This is particularly useful when using the [code execution tool](/docs/en/agents-and-tools/tool-use/code-execution-tool) to provide inputs (e.g. datasets and documents) and then download outputs (e.g. charts). You can also use the Files API to prevent having to continually re-upload frequently used documents and images across multiple API calls. You can [explore the API reference directly](/docs/en/api/files-create), in addition to this guide.



The Files API is in beta. Reach out through the [feedback form](https://forms.gle/tisHyierGwgN4DUE9) to share your experience with the Files API.



This feature is **not** eligible for [Zero Data Retention (ZDR)](/docs/en/build-with-claude/api-and-data-retention). Data is retained according to the feature's standard retention policy.

## Supported models

Referencing a `file_id` in a Messages request is supported on all models that support the given file type. [Images](/docs/en/build-with-claude/vision) are supported on all current Claude models. For [PDFs](/docs/en/build-with-claude/pdf-support) and [other file types with the code execution tool](/docs/en/agents-and-tools/tool-use/code-execution-tool#model-compatibility), see the linked pages for model support.

The Files API is available on the Claude API, [Claude Platform on AWS](/docs/en/build-with-claude/claude-platform-on-aws), and [Microsoft Foundry](/docs/en/build-with-claude/claude-in-microsoft-foundry). On Microsoft Foundry, the Files API requires a [Hosted on Anthropic deployment](/docs/en/build-with-claude/claude-in-microsoft-foundry#additional-features-not-supported-when-hosted-on-azure). It is not currently available on Amazon Bedrock or Google Cloud.

## How the Files API works

The Files API provides a simple create-once, use-many-times approach for working with files:

* **Upload files** to Anthropic's secure storage and receive a unique `file_id`
* **Download files** that are created from skills or the code execution tool
* **Reference files** in [Messages](/docs/en/api/messages/create) requests using the `file_id` instead of re-uploading content
* **Manage your files** with list, retrieve, and delete operations

## How to use the Files API



To use the Files API, you'll need to include the beta feature header: `anthropic-beta: files-api-2025-04-14`.

### Uploading a file

Upload a file to be referenced in future API calls:

```
uploaded = client.beta.files.upload(uploaded = client.beta.files.upload( file=("document.pdf", open("/path/to/document.pdf", "rb"), "application/pdf"),  file =("document.pdf", open("/path/to/document.pdf", "rb"), "application/pdf"),))
```

The response from uploading a file will include:

Output

```
{{ "id": "file_011CNha8iCJcU1wXNR6q4V8w",  "id": "file_011CNha8iCJcU1wXNR6q4V8w", "type": "file",  "type": "file", "filename": "document.pdf",  "filename": "document.pdf", "mime_type": "application/pdf",  "mime_type": "application/pdf", "size_bytes": 1024000,  "size_bytes": 1024000, "created_at": "2025-01-01T00:00:00Z",  "created_at": "2025-01-01T00:00:00Z", "downloadable": false  "downloadable": false}}
```

### Using a file in messages

Once uploaded, reference the file using its `file_id`:

```
response = client.beta.messages.create(response = client.beta.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, messages=[ messages =[ { { "role": "user",  "role": "user", "content": [ "content": [ {"type": "text", "text": "Please summarize this document for me."}, {"type": "text", "text": "Please summarize this document for me."}, { { "type": "document",  "type": "document", "source": { "source": { "type": "file",  "type": "file", "file_id": file_id,  "file_id": file_id, }, }, }, }, ], ], } } ], ], betas=["files-api-2025-04-14"],  betas =["files-api-2025-04-14"],))print(response) print(response)
```

### File types and content blocks

The Files API supports different file types that correspond to different content block types:

| File Type | MIME Type | Content Block Type | Use Case |
| --- | --- | --- | --- |
| PDF | `application/pdf` | `document` | Text analysis, document processing |
| Plain text | `text/plain` | `document` | Text analysis, processing |
| Images | `image/jpeg`, `image/png`, `image/gif`, `image/webp` | `image` | Image analysis, visual tasks |
| [Datasets, others](/docs/en/agents-and-tools/tool-use/code-execution-tool#upload-and-analyze-your-own-files) | Varies | `container_upload` | Analyze data, create visualizations |

### Working with other file formats

For file types that are not supported as `document` blocks (.csv, .txt, .md, .docx, .xlsx), convert the files to plain text, and include the content directly in your message:

```
import pandas as pd import  pandas as  pd# ...# ...# Example: Reading a CSV file# Example: Reading a CSV filedf = pd.read_csv("data.csv") df = pd.read_csv("data.csv")csv_content = df.to_string() csv_content = df.to_string() # Send as plain text in the message # Send as plain text in the messageresponse = client.messages.create(response = client.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, messages=[ messages =[ { { "role": "user",  "role": "user", "content": [ "content": [ { { "type": "text",  "type": "text", "text": f"Here's the CSV data:\n\n{csv_content}\n\nPlease analyze this data.",  "text": f"Here's the CSV data: \n\n{csv_content} \n\nPlease analyze this data.", } } ], ], } } ], ],)) print(response.content[0].text) print(response.content[0].text)
```



For .docx files containing images, convert them to PDF format first, then use [PDF support](/docs/en/build-with-claude/pdf-support) to take advantage of the built-in image parsing. This allows using citations from the PDF document.

#### Document blocks

For PDFs and text files, use the `document` content block:

```
{{ "type": "document",  "type": "document", "source": { "source": { "type": "file",  "type": "file", "file_id": "file_011CNha8iCJcU1wXNR6q4V8w"  "file_id": "file_011CNha8iCJcU1wXNR6q4V8w" }, }, "title": "Document Title", // Optional  "title": "Document Title", // Optional "context": "Context about the document", // Optional  "context": "Context about the document", // Optional "citations": { "enabled": true } // Optional, enables citations  "citations": { "enabled": true } // Optional, enables citations}}
```

#### Image blocks

For images, use the `image` content block:

```
{{ "type": "image",  "type": "image", "source": { "source": { "type": "file",  "type": "file", "file_id": "file_011CPMxVD3fHLUhvTqtsQA5w"  "file_id": "file_011CPMxVD3fHLUhvTqtsQA5w" } }}}
```

### Managing files

#### List files

Retrieve a list of your uploaded files:

```
client = anthropic.Anthropic() client = anthropic.Anthropic()files = client.beta.files.list() files = client.beta.files.list()
```

#### Get file metadata

Retrieve information about a specific file:

```
file = client.beta.files.retrieve_metadata(file_id) file = client.beta.files.retrieve_metadata(file_id)
```

#### Delete a file

Remove a file from your workspace:

```
result = client.beta.files.delete(file_id) result = client.beta.files.delete(file_id)
```

### Downloading a file

Download files that have been created by skills or the code execution tool:

```
file_content = client.beta.files.download(file_id) file_content = client.beta.files.download(file_id) # Save to file # Save to filefile_content.write_to_file("downloaded_file.txt")file_content.write_to_file("downloaded_file.txt")
```



You can only download files that were created by [skills](/docs/en/build-with-claude/skills-guide) or the [code execution tool](/docs/en/agents-and-tools/tool-use/code-execution-tool). Files that you uploaded cannot be downloaded.

---

## File storage and limits

### Storage limits

* **Maximum file size:** 500 MB per file
* **Total storage:** 500 GB per organization

### File lifecycle

* Files are scoped to the workspace of the API key. Other API keys can use files created by any other API key associated with the same workspace
* Files persist until you delete them
* Deleted files cannot be recovered
* Files are inaccessible via the API shortly after deletion, but they may persist in active `Messages` API calls and associated tool uses
* Files that users delete will be deleted in accordance with Anthropic's [data retention policy](https://privacy.claude.com/en/articles/7996866-how-long-do-you-store-my-organization-s-data).

---

## Data retention

Files uploaded via the Files API are retained until explicitly deleted using the `DELETE /v1/files/{file_id}` endpoint. Files are stored for reuse across multiple API requests.

For ZDR eligibility across all features, see [API and data retention](/docs/en/manage-claude/api-and-data-retention).

## Error handling

Common errors when using the Files API include:

* **File not found (404):** The specified `file_id` doesn't exist or you don't have access to it
* **Invalid file type (400):** The file type doesn't match the content block type (e.g., using an image file in a document block)
* **Exceeds context window size (400):** The file is larger than the context window size (e.g. using a 500 MB plaintext file in a `/v1/messages` request)
* **Invalid filename (400):** Filename doesn't meet the length requirements (1-255 characters) or contains forbidden characters (`<`, `>`, `:`, `"`, `|`, `?`, `*`, `\`, `/`, or unicode characters 0-31)
* **File too large (413):** File exceeds the 500 MB limit
* **Storage limit exceeded (403):** Your organization has reached the 500 GB storage limit

Output

```
{{ "type": "error",  "type": "error", "error": { "error": { "type": "invalid_request_error",  "type": "invalid_request_error", "message": "File not found: file_011CNha8iCJcU1wXNR6q4V8w"  "message": "File not found: file_011CNha8iCJcU1wXNR6q4V8w" } }}}
```

## Usage and billing

File API operations are **free**:

* Uploading files
* Downloading files
* Listing files
* Getting file metadata
* Deleting files

File content used in `Messages` requests are priced as input tokens. You can only download files created by [skills](/docs/en/build-with-claude/skills-guide) or the [code execution tool](/docs/en/agents-and-tools/tool-use/code-execution-tool).

### Rate limits

During the beta period:

* File-related API calls are limited to approximately 100 requests per minute
* [Contact us](/cdn-cgi/l/email-protection#90e3f1fcf5e3d0f1fee4f8e2ffe0f9f3bef3fffd) if you need higher limits for your use case

Was this page helpful?
