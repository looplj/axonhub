# Files API

---

The Files API lets you upload and manage files to use with the Claude API without re-uploading content with each request. This is particularly useful when using the [code execution tool](/docs/en/agents-and-tools/tool-use/code-execution-tool) to provide inputs (e.g. datasets and documents) and then download outputs (e.g. charts). You can also use the Files API to prevent having to continually re-upload frequently used documents and images across multiple API calls. You can [explore the API reference directly](/docs/en/api/files-create), in addition to this guide.

<Note>
  The Files API is in beta. Reach out through the [feedback form](https://forms.gle/tisHyierGwgN4DUE9) to share your experience with the Files API.
</Note>

<Note>
  This feature is **not** eligible for [Zero Data Retention (ZDR)](/docs/en/build-with-claude/api-and-data-retention). Data is retained according to the feature's standard retention policy.
</Note>

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

<Note>
  To use the Files API, you'll need to include the beta feature header: `anthropic-beta: files-api-2025-04-14`.
</Note>

### Uploading a file

Upload a file to be referenced in future API calls:

<CodeGroup>
  ```bash cURL
  curl -X POST https://api.anthropic.com/v1/files \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -H "anthropic-beta: files-api-2025-04-14" \
    -F "file=@/path/to/document.pdf"
  ```

  ```bash CLI
  FILE_ID=$(ant beta:files upload \
    --file /path/to/document.pdf \
    --transform id --raw-output)
  ```

  ```python Python
  uploaded = client.beta.files.upload(
      file=("document.pdf", open("/path/to/document.pdf", "rb"), "application/pdf"),
  )
  ```

  ```typescript TypeScript
  const uploaded = await client.beta.files.upload({
    file: await toFile(
      fs.createReadStream("/path/to/document.pdf"),
      undefined,
      { type: "application/pdf" },
    ),
  });
  ```

  ```csharp C#
  var uploaded = await client.Beta.Files.Upload(
      new FileUploadParams
      {
          File = new BinaryContent
          {
              Stream = File.OpenRead("/path/to/document.pdf"),
              FileName = "document.pdf",
              ContentType = new("application/pdf")
          }
      });

  Console.WriteLine(uploaded.ID);
  ```

  ```go Go
  f, err := os.Open("/path/to/document.pdf")
  if err != nil {
  	log.Fatal(err)
  }
  defer f.Close()

  response, err := client.Beta.Files.Upload(context.Background(),
  	anthropic.BetaFileUploadParams{
  		File: anthropic.File(f, "document.pdf", "application/pdf"),
  	})
  if err != nil {
  	log.Fatal(err)
  }

  fmt.Println(response.ID)
  ```

  ```java Java
  FileMetadata file = client.beta().files().upload(
      FileUploadParams.builder()
          .file(MultipartField.<InputStream>builder()
              .value(Files.newInputStream(Path.of("/path/to/document.pdf")))
              .filename("document.pdf")
              .contentType("application/pdf")
              .build())
          .build()
  );

  System.out.println(file.id());
  ```

  ```php PHP
  $file = $client->beta->files->upload(
      FileParam::fromResource(fopen('/path/to/document.pdf', 'rb'), contentType: 'application/pdf'),
  );

  echo $file->id;
  ```

  ```ruby Ruby
  file = client.beta.files.upload(
    file: Anthropic::FilePart.new(
      Pathname("/path/to/document.pdf"),
      content_type: "application/pdf"
    )
  )

  puts file.id
  ```
</CodeGroup>

The response from uploading a file will include:

```json Output
{
  "id": "file_011CNha8iCJcU1wXNR6q4V8w",
  "type": "file",
  "filename": "document.pdf",
  "mime_type": "application/pdf",
  "size_bytes": 1024000,
  "created_at": "2025-01-01T00:00:00Z",
  "downloadable": false
}
```

### Using a file in messages

Once uploaded, reference the file using its `file_id`:

<CodeGroup>
  ```bash cURL
  curl -X POST https://api.anthropic.com/v1/messages \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -H "anthropic-beta: files-api-2025-04-14" \
    -H "content-type: application/json" \
    -d @- <<EOF
  {
    "model": "claude-opus-4-8",
    "max_tokens": 1024,
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "Please summarize this document for me."
          },
          {
            "type": "document",
            "source": {
              "type": "file",
              "file_id": "$FILE_ID"
            }
          }
        ]
      }
    ]
  }
  EOF
  ```

  ```bash CLI
  ant beta:messages create --beta files-api-2025-04-14 <<YAML
  model: claude-opus-4-8
  max_tokens: 1024
  messages:
    - role: user
      content:
        - type: text
          text: Please summarize this document for me.
        - type: document
          source:
            type: file
            file_id: $FILE_ID
  YAML
  ```

  ```python Python
  response = client.beta.messages.create(
      model="claude-opus-4-8",
      max_tokens=1024,
      messages=[
          {
              "role": "user",
              "content": [
                  {"type": "text", "text": "Please summarize this document for me."},
                  {
                      "type": "document",
                      "source": {
                          "type": "file",
                          "file_id": file_id,
                      },
                  },
              ],
          }
      ],
      betas=["files-api-2025-04-14"],
  )
  print(response)
  ```

  ```typescript TypeScript
  const response = await client.beta.messages.create({
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: "Please summarize this document for me.",
          },
          {
            type: "document",
            source: {
              type: "file",
              file_id: uploaded.id,
            },
          },
        ],
      },
    ],
    betas: ["files-api-2025-04-14"],
  });

  console.log(response);
  ```

  ```csharp C#
  var response = await client.Beta.Messages.Create(
      new MessageCreateParams
      {
          Model = Messages::Model.ClaudeOpus4_8,
          MaxTokens = 1024,
          Betas = [AnthropicBeta.FilesApi2025_04_14],
          Messages =
          [
              new BetaMessageParam
              {
                  Role = Role.User,
                  Content = new List<BetaContentBlockParam>
                  {
                      new BetaTextBlockParam { Text = "Please summarize this document for me." },
                      new BetaRequestDocumentBlock
                      {
                          Source = new BetaFileDocumentSource { FileID = fileId }
                      }
                  }
              }
          ]
      });

  Console.WriteLine(response);
  ```

  ```go Go
  msg, err := client.Beta.Messages.New(context.Background(),
  	anthropic.BetaMessageNewParams{
  		Model:     anthropic.ModelClaudeOpus4_8,
  		MaxTokens: 1024,
  		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaFilesAPI2025_04_14},
  		Messages: []anthropic.BetaMessageParam{
  			anthropic.NewBetaUserMessage(
  				anthropic.NewBetaTextBlock("Please summarize this document for me."),
  				anthropic.NewBetaDocumentBlock(anthropic.BetaFileDocumentSourceParam{
  					FileID: fileID,
  				}),
  			),
  		},
  	})
  if err != nil {
  	log.Fatal(err)
  }

  fmt.Println(msg)
  ```

  ```java Java
  MessageCreateParams params = MessageCreateParams.builder()
      .model(Model.CLAUDE_OPUS_4_8)
      .addBeta("files-api-2025-04-14")
      .maxTokens(1024)
      .addUserMessageOfBetaContentBlockParams(List.of(
          BetaContentBlockParam.ofText(BetaTextBlockParam.builder()
              .text("Please summarize this document for me.")
              .build()),
          BetaContentBlockParam.ofDocument(BetaRequestDocumentBlock.builder()
              .source(BetaFileDocumentSource.builder()
                  .fileId(fileId)
                  .build())
              .build())
      ))
      .build();

  BetaMessage message = client.beta().messages().create(params);
  System.out.println(message);
  ```

  ```php PHP
  $response = $client->beta->messages->create(
      maxTokens: 1024,
      messages: [
          [
              'role' => 'user',
              'content' => [
                  ['type' => 'text', 'text' => 'Please summarize this document for me.'],
                  [
                      'type' => 'document',
                      'source' => [
                          'type' => 'file',
                          'file_id' => $fileId
                      ]
                  ]
              ]
          ]
      ],
      model: 'claude-opus-4-8',
      betas: ['files-api-2025-04-14'],
  );

  print_r($response);
  ```

  ```ruby Ruby
  response = client.beta.messages.create(
    model: "claude-opus-4-8",
    max_tokens: 1024,
    betas: ["files-api-2025-04-14"],
    messages: [
      {
        role: "user",
        content: [
          { type: "text", text: "Please summarize this document for me." },
          {
            type: "document",
            source: {
              type: "file",
              file_id: file_id
            }
          }
        ]
      }
    ]
  )

  puts response
  ```
</CodeGroup>

### File types and content blocks

The Files API supports different file types that correspond to different content block types:

| File Type                                                                                                    | MIME Type                                            | Content Block Type | Use Case                            |
| ------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------- | ------------------ | ----------------------------------- |
| PDF                                                                                                          | `application/pdf`                                    | `document`         | Text analysis, document processing  |
| Plain text                                                                                                   | `text/plain`                                         | `document`         | Text analysis, processing           |
| Images                                                                                                       | `image/jpeg`, `image/png`, `image/gif`, `image/webp` | `image`            | Image analysis, visual tasks        |
| [Datasets, others](/docs/en/agents-and-tools/tool-use/code-execution-tool#upload-and-analyze-your-own-files) | Varies                                               | `container_upload` | Analyze data, create visualizations |

### Working with other file formats

For file types that are not supported as `document` blocks (.csv, .txt, .md, .docx, .xlsx), convert the files to plain text, and include the content directly in your message:

<CodeGroup>
  ```bash cURL
  # Example: Reading a text file and sending it as plain text
  # Note: For files with special characters, consider base64 encoding
  # ...
  curl https://api.anthropic.com/v1/messages \
    -H "content-type: application/json" \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -d @- <<EOF
  {
    "model": "claude-opus-4-8",
    "max_tokens": 1024,
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "Here's the document content:\n\n${TEXT_CONTENT}\n\nPlease summarize this document."
          }
        ]
      }
    ]
  }
  EOF
  ```

  ```bash CLI
  # The "@./path" reference inlines the file contents directly into the field.
  ant messages create \
    --model claude-opus-4-8 \
    --max-tokens 1024 \
    --transform 'content.0.text' --raw-output <<'YAML'
  messages:
    - role: user
      content:
        - type: text
          text: "Here's the document content:"
        - type: text
          text: "@./document.txt"
        - type: text
          text: "Please summarize this document."
  YAML
  ```

  ```python Python
  import pandas as pd
  # ...
  # Example: Reading a CSV file
  df = pd.read_csv("data.csv")
  csv_content = df.to_string()

  # Send as plain text in the message
  response = client.messages.create(
      model="claude-opus-4-8",
      max_tokens=1024,
      messages=[
          {
              "role": "user",
              "content": [
                  {
                      "type": "text",
                      "text": f"Here's the CSV data:\n\n{csv_content}\n\nPlease analyze this data.",
                  }
              ],
          }
      ],
  )

  print(response.content[0].text)
  ```

  ```typescript TypeScript
  import fs from "node:fs/promises";

  const anthropic = new Anthropic();

  async function analyzeDocument() {
    // Example: Reading a text file
    const textContent = await fs.readFile("document.txt", "utf-8");

    // Send as plain text in the message
    const response = await anthropic.messages.create({
      model: "claude-opus-4-8",
      max_tokens: 1024,
      messages: [
        {
          role: "user",
          content: [
            {
              type: "text",
              text: `Here's the document content:\n\n${textContent}\n\nPlease summarize this document.`
            }
          ]
        }
      ]
    });

    const block = response.content[0];
    if (block.type === "text") {
      console.log(block.text);
    }
  }

  analyzeDocument();
  ```

  ```csharp C#
  using System;
  using System.IO;
  using System.Threading.Tasks;
  using Anthropic;
  using Anthropic.Models.Messages;

  class Program
  {
      static async Task Main(string[] args)
      {
          AnthropicClient client = new();

          // Example: Reading a text file
          string textContent = await File.ReadAllTextAsync("document.txt");

          var parameters = new MessageCreateParams
          {
              Model = Model.ClaudeOpus4_8,
              MaxTokens = 1024,
              Messages = [new()
              {
                  Role = Role.User,
                  Content = $"Here's the document content:\n\n{textContent}\n\nPlease summarize this document."
              }]
          };

          var message = await client.Messages.Create(parameters);
          Console.WriteLine(message);
      }
  }
  ```

  ```go Go
  package main

  import (
  	"context"
  	"fmt"
  	"log"
  	"os"

  	"github.com/anthropics/anthropic-sdk-go"
  )
  // ...
  func main() {
  	client := anthropic.NewClient()

  	// Example: Reading a text file
  	textContent, err := os.ReadFile("document.txt")
  	if err != nil {
  		log.Fatal(err)
  	}

  	response, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
  		Model:     anthropic.ModelClaudeOpus4_8,
  		MaxTokens: 1024,
  		Messages: []anthropic.MessageParam{
  			anthropic.NewUserMessage(anthropic.NewTextBlock(
  				fmt.Sprintf("Here's the document content:\n\n%s\n\nPlease summarize this document.", string(textContent)),
  			)),
  		},
  	})
  	if err != nil {
  		log.Fatal(err)
  	}

  	fmt.Println(response.Content[0].Text)
  }
  ```

  ```java Java
  AnthropicClient client = AnthropicOkHttpClient.fromEnv();

  // Example: Reading a text file
  String textContent = Files.readString(Paths.get("document.txt"));

  MessageCreateParams params = MessageCreateParams.builder()
      .model(Model.CLAUDE_OPUS_4_8)
      .maxTokens(1024L)
      .addUserMessage("Here's the document content:\n\n" + textContent + "\n\nPlease summarize this document.")
      .build();

  Message response = client.messages().create(params);
  response.content().stream()
      .flatMap(block -> block.text().stream())
      .forEach(textBlock -> System.out.println(textBlock.text()));
  ```

  ```php PHP
  $client = new Client();

  // Example: Reading a text file
  $textContent = file_get_contents("document.txt");

  $message = $client->messages->create(
      maxTokens: 1024,
      messages: [
          [
              'role' => 'user',
              'content' => [
                  [
                      'type' => 'text',
                      'text' => "Here's the document content:\n\n{$textContent}\n\nPlease summarize this document."
                  ]
              ]
          ]
      ],
      model: 'claude-opus-4-8',
  );

  echo $message->content[0]->text;
  ```

  ```ruby Ruby
  client = Anthropic::Client.new

  # Example: Reading a text file
  text_content = File.read("document.txt")

  message = client.messages.create(
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: "Here's the document content:\n\n#{text_content}\n\nPlease summarize this document."
          }
        ]
      }
    ]
  )

  puts message.content.first.text
  ```
</CodeGroup>

<Note>
  For .docx files containing images, convert them to PDF format first, then use [PDF support](/docs/en/build-with-claude/pdf-support) to take advantage of the built-in image parsing. This allows using citations from the PDF document.
</Note>

#### Document blocks

For PDFs and text files, use the `document` content block:

```json
{
  "type": "document",
  "source": {
    "type": "file",
    "file_id": "file_011CNha8iCJcU1wXNR6q4V8w"
  },
  "title": "Document Title", // Optional
  "context": "Context about the document", // Optional
  "citations": { "enabled": true } // Optional, enables citations
}
```

#### Image blocks

For images, use the `image` content block:

```json
{
  "type": "image",
  "source": {
    "type": "file",
    "file_id": "file_011CPMxVD3fHLUhvTqtsQA5w"
  }
}
```

### Managing files

#### List files

Retrieve a list of your uploaded files:

<CodeGroup>
  ```bash cURL
  curl https://api.anthropic.com/v1/files \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -H "anthropic-beta: files-api-2025-04-14"
  ```

  ```bash CLI
  ant beta:files list
  ```

  ```python Python
  client = anthropic.Anthropic()
  files = client.beta.files.list()
  ```

  ```typescript TypeScript
  const anthropic = new Anthropic();
  const files = await anthropic.beta.files.list();
  ```

  ```csharp C#
  using System;
  using System.Threading.Tasks;
  using Anthropic;
  using Anthropic.Models.Beta.Files;

  class Program
  {
      static async Task Main(string[] args)
      {
          AnthropicClient client = new();

          var files = await client.Beta.Files.List();
          Console.WriteLine(files);
      }
  }
  ```

  ```go Go
  client := anthropic.NewClient()

  files, err := client.Beta.Files.List(context.TODO(), anthropic.BetaFileListParams{})
  if err != nil {
  	log.Fatal(err)
  }
  fmt.Println(files)
  ```

  ```java Java
  import com.anthropic.models.beta.files.FileListPage;
  // ...
          AnthropicClient client = AnthropicOkHttpClient.fromEnv();

          FileListPage files = client.beta().files().list();
          System.out.println(files);
  ```

  ```php PHP
  $client = new Client();

  $files = $client->beta->files->list();
  print_r($files);
  ```

  ```ruby Ruby
  client = Anthropic::Client.new

  files = client.beta.files.list
  puts files
  ```
</CodeGroup>

#### Get file metadata

Retrieve information about a specific file:

<CodeGroup>
  ```bash cURL
  curl "https://api.anthropic.com/v1/files/$FILE_ID" \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -H "anthropic-beta: files-api-2025-04-14"
  ```

  ```bash CLI
  ant beta:files retrieve-metadata \
    --file-id "$FILE_ID"
  ```

  ```python Python
  file = client.beta.files.retrieve_metadata(file_id)
  ```

  ```typescript TypeScript
  const file = await client.beta.files.retrieveMetadata(uploaded.id);
  ```

  ```csharp C#
  var file = await client.Beta.Files.RetrieveMetadata(fileId);
  Console.WriteLine(file);
  ```

  ```go Go
  metadata, err := client.Beta.Files.GetMetadata(
  	context.TODO(),
  	fileID,
  	anthropic.BetaFileGetMetadataParams{},
  )
  if err != nil {
  	log.Fatal(err)
  }

  fmt.Println(metadata)
  ```

  ```java Java
  FileMetadata metadata = client.beta().files().retrieveMetadata(fileId);

  System.out.println(metadata);
  ```

  ```php PHP
  $file = $client->beta->files->retrieveMetadata($fileId);
  echo $file;
  ```

  ```ruby Ruby
  file = client.beta.files.retrieve_metadata(file_id)
  puts file
  ```
</CodeGroup>

#### Delete a file

Remove a file from your workspace:

<CodeGroup>
  ```bash cURL
  curl -X DELETE "https://api.anthropic.com/v1/files/$FILE_ID" \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -H "anthropic-beta: files-api-2025-04-14"
  ```

  ```bash CLI
  ant beta:files delete \
    --file-id "$FILE_ID"
  ```

  ```python Python
  result = client.beta.files.delete(file_id)
  ```

  ```typescript TypeScript
  await client.beta.files.delete(uploaded.id);
  ```

  ```csharp C#
  await client.Beta.Files.Delete(fileId);
  ```

  ```go Go
  _, err = client.Beta.Files.Delete(
  	context.TODO(),
  	fileID,
  	anthropic.BetaFileDeleteParams{},
  )
  if err != nil {
  	log.Fatal(err)
  }
  ```

  ```java Java
  client.beta().files().delete(fileId);
  ```

  ```php PHP
  $result = $client->beta->files->delete($fileId);
  ```

  ```ruby Ruby
  result = client.beta.files.delete(file_id)
  ```
</CodeGroup>

### Downloading a file

Download files that have been created by skills or the code execution tool:

<CodeGroup>
  ```bash cURL
  curl -X GET "https://api.anthropic.com/v1/files/$FILE_ID/content" \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -H "anthropic-beta: files-api-2025-04-14" \
    --output downloaded_file.txt
  ```

  ```bash CLI
  ant beta:files download \
    --file-id "$FILE_ID" \
    --output downloaded_file.txt
  ```

  ```python Python
  file_content = client.beta.files.download(file_id)

  # Save to file
  file_content.write_to_file("downloaded_file.txt")
  ```

  ```typescript TypeScript
  const content = await client.beta.files.download(uploaded.id);

  const bytes = Buffer.from(await content.arrayBuffer());
  await fsp.writeFile("downloaded_file.txt", bytes);
  ```

  ```csharp C#
  using var fileContent = await client.Beta.Files.Download(fileId);
  await using var source = await fileContent.ReadAsStream();
  await using var destination = File.Create("downloaded_file.txt");
  await source.CopyToAsync(destination);
  ```

  ```go Go
  func downloadFile(client anthropic.Client, fileID string) error {
  	resp, err := client.Beta.Files.Download(
  		context.TODO(),
  		fileID,
  		anthropic.BetaFileDownloadParams{},
  	)
  	if err != nil {
  		return err
  	}
  	defer resp.Body.Close()

  	fileContent, err := io.ReadAll(resp.Body)
  	if err != nil {
  		return err
  	}

  	return os.WriteFile("downloaded_file.txt", fileContent, 0644)
  }

  ```

  ```java Java
  try (HttpResponse response = client.beta().files().download(fileId)) {
      try (InputStream body = response.body()) {
          Files.copy(body, Path.of("downloaded_file.txt"),
              StandardCopyOption.REPLACE_EXISTING);
      }
  }
  ```

  ```php PHP
  $fileContent = $client->beta->files->download($fileId);

  file_put_contents("downloaded_file.txt", $fileContent);
  ```

  ```ruby Ruby
  file_content = client.beta.files.download(file_id)

  File.binwrite("downloaded_file.txt", file_content.read)
  ```
</CodeGroup>

<Note>
  You can only download files that were created by [skills](/docs/en/build-with-claude/skills-guide) or the [code execution tool](/docs/en/agents-and-tools/tool-use/code-execution-tool). Files that you uploaded cannot be downloaded.
</Note>

***

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

***

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

```json Output
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "File not found: file_011CNha8iCJcU1wXNR6q4V8w"
  }
}
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
* [Contact us](mailto:sales@anthropic.com) if you need higher limits for your use case
