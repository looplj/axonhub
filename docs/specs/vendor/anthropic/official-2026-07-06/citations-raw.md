# Citations

Ground Claude's responses in your source documents. Citations return the exact passages that support each claim, so you can verify answers and surface sources to your users.

---

<Note>
  This feature is eligible for [Zero Data Retention (ZDR)](/docs/en/build-with-claude/api-and-data-retention). When your organization has a ZDR arrangement, data sent through this feature is not stored after the API response is returned.
</Note>

Claude can provide detailed citations when answering questions about documents, helping you track and verify the sources behind each response.

All [active models](/docs/en/about-claude/models/overview) support citations, with the exception of Claude Haiku 3.

<Tip>
  Share your feedback and suggestions about the citations feature using the [citations feedback form](https://forms.gle/9n9hSrKnKe3rpowH9).
</Tip>

The following example shows how to enable citations on a plain text document with the Messages API:

<CodeGroup>
  ```bash cURL
  curl https://api.anthropic.com/v1/messages \
    -H "content-type: application/json" \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -d '{
      "model": "claude-opus-4-8",
      "max_tokens": 1024,
      "messages": [
        {
          "role": "user",
          "content": [
            {
              "type": "document",
              "source": {
                "type": "text",
                "media_type": "text/plain",
                "data": "The grass is green. The sky is blue."
              },
              "title": "My Document",
              "context": "This is a trustworthy document.",
              "citations": {"enabled": true}
            },
            {
              "type": "text",
              "text": "What color is the grass and sky?"
            }
          ]
        }
      ]
    }'
  ```

  ```bash CLI
  ant messages create <<'YAML'
  model: claude-opus-4-8
  max_tokens: 1024
  messages:
    - role: user
      content:
        - type: document
          source:
            type: text
            media_type: text/plain
            data: The grass is green. The sky is blue.
          title: My Document
          context: This is a trustworthy document.
          citations:
            enabled: true
        - type: text
          text: What color is the grass and sky?
  YAML
  ```

  ```python Python
  client = anthropic.Anthropic()

  response = client.messages.create(
      model="claude-opus-4-8",
      max_tokens=1024,
      messages=[
          {
              "role": "user",
              "content": [
                  {
                      "type": "document",
                      "source": {
                          "type": "text",
                          "media_type": "text/plain",
                          "data": "The grass is green. The sky is blue.",
                      },
                      "title": "My Document",
                      "context": "This is a trustworthy document.",
                      "citations": {"enabled": True},
                  },
                  {"type": "text", "text": "What color is the grass and sky?"},
              ],
          }
      ],
  )
  print(response)
  ```

  ```typescript TypeScript
  const client = new Anthropic();

  const response = await client.messages.create({
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "document",
            source: {
              type: "text",
              media_type: "text/plain",
              data: "The grass is green. The sky is blue."
            },
            title: "My Document",
            context: "This is a trustworthy document.",
            citations: { enabled: true }
          },
          {
            type: "text",
            text: "What color is the grass and sky?"
          }
        ]
      }
    ]
  });
  console.log(response);
  ```

  ```csharp C#
  var client = new AnthropicClient();

  var response = await client.Messages.Create(
      new()
      {
          Model = Model.ClaudeOpus4_8,
          MaxTokens = 1024,
          Messages =
          [
              new()
              {
                  Role = Role.User,
                  Content = new MessageParamContent(new List<ContentBlockParam>
                  {
                      new ContentBlockParam(new DocumentBlockParam(
                          new DocumentBlockParamSource(new PlainTextSource()
                          {
                              Data = "The grass is green. The sky is blue.",
                          })
                      )
                      {
                          Title = "My Document",
                          Context = "This is a trustworthy document.",
                          Citations = new CitationsConfigParam { Enabled = true },
                      }),
                      new ContentBlockParam(new TextBlockParam("What color is the grass and sky?")),
                  }),
              },
          ],
      }
  );

  Console.WriteLine(response);
  ```

  ```go Go
  client := anthropic.NewClient()

  response, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
  	Model:     anthropic.ModelClaudeOpus4_8,
  	MaxTokens: 1024,
  	Messages: []anthropic.MessageParam{
  		anthropic.NewUserMessage(
  			anthropic.ContentBlockParamUnion{
  				OfDocument: &anthropic.DocumentBlockParam{
  					Source: anthropic.DocumentBlockParamSourceUnion{
  						OfText: &anthropic.PlainTextSourceParam{
  							Data: "The grass is green. The sky is blue.",
  						},
  					},
  					Title:     anthropic.String("My Document"),
  					Context:   anthropic.String("This is a trustworthy document."),
  					Citations: anthropic.CitationsConfigParam{Enabled: anthropic.Bool(true)},
  				},
  			},
  			anthropic.NewTextBlock("What color is the grass and sky?"),
  		),
  	},
  })
  if err != nil {
  	log.Fatal(err)
  }
  fmt.Println(response)
  ```

  ```java Java
  AnthropicClient client = AnthropicOkHttpClient.fromEnv();

  PlainTextSource source = PlainTextSource.builder()
    .data("The grass is green. The sky is blue.")
    .build();

  DocumentBlockParam documentParam = DocumentBlockParam.builder()
    .source(source)
    .title("My Document")
    .context("This is a trustworthy document.")
    .citations(CitationsConfigParam.builder().enabled(true).build())
    .build();

  TextBlockParam textBlockParam = TextBlockParam.builder()
    .text("What color is the grass and sky?")
    .build();

  MessageCreateParams params = MessageCreateParams.builder()
    .model(Model.CLAUDE_OPUS_4_8)
    .maxTokens(1024)
    .addUserMessageOfBlockParams(
      List.of(
        ContentBlockParam.ofDocument(documentParam),
        ContentBlockParam.ofText(textBlockParam)
      )
    )
    .build();

  Message message = client.messages().create(params);
  System.out.println(message);
  ```

  ```php PHP
  $client = new Client();

  $response = $client->messages->create(
      maxTokens: 1024,
      messages: [
          [
              'role' => 'user',
              'content' => [
                  [
                      'type' => 'document',
                      'source' => [
                          'type' => 'text',
                          'media_type' => 'text/plain',
                          'data' => 'The grass is green. The sky is blue.',
                      ],
                      'title' => 'My Document',
                      'context' => 'This is a trustworthy document.',
                      'citations' => ['enabled' => true],
                  ],
                  [
                      'type' => 'text',
                      'text' => 'What color is the grass and sky?',
                  ],
              ],
          ],
      ],
      model: 'claude-opus-4-8',
  );

  echo json_encode($response, JSON_PRETTY_PRINT);
  ```

  ```ruby Ruby
  client = Anthropic::Client.new

  response = client.messages.create(
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "document",
            source: {
              type: "text",
              media_type: "text/plain",
              data: "The grass is green. The sky is blue."
            },
            title: "My Document",
            context: "This is a trustworthy document.",
            citations: { enabled: true }
          },
          {
            type: "text",
            text: "What color is the grass and sky?"
          }
        ]
      }
    ]
  )

  puts response
  ```
</CodeGroup>

<Tip>
  **Comparison with prompt-based approaches**

  Compared to prompting Claude to cite sources, the citations feature offers the following advantages:

  * **Cost savings:** If your prompt-based approach asks Claude to output direct quotes, you may see cost savings because `cited_text` does not count toward your output tokens.
  * **Better citation reliability:** Because the API parses citations into the response formats described in the following sections and extracts `cited_text` directly, citations are guaranteed to contain valid pointers to the provided documents.
  * **Improved citation quality:** In Anthropic's evaluations, the citations feature is significantly more likely to cite the most relevant quotes from documents than purely prompt-based approaches.
</Tip>

***

## How citations work

Integrate citations with Claude in these steps:

<Steps>
  <Step title="Provide document(s) and enable citations">
    * Include documents in any of the supported formats: [PDFs](#pdf-documents), [plain text](#plain-text-documents), or [custom content](#custom-content-documents) documents.
    * Set `citations.enabled=true` on each of your documents. Currently, citations must be enabled on all or none of the documents within a request.
    * Only text citations are currently supported. Image citations are not yet possible.
  </Step>

  <Step title="Documents get processed">
    * Document contents are "chunked" to define the minimum granularity of possible citations. For example, sentence chunking lets Claude cite a single sentence or chain together multiple consecutive sentences to cite a paragraph or longer passage.

      * **For PDFs:** Text is extracted as described in [PDF support](/docs/en/build-with-claude/pdf-support) and content is chunked into sentences. Citing images from PDFs is not currently supported.
      * **For plain text documents:** Content is chunked into sentences that can be cited from.
      * **For custom content documents:** Your provided content blocks are used as-is and no further chunking is done.
  </Step>

  <Step title="Claude provides cited response">
    * Responses may now include multiple text blocks where each text block can contain a claim that Claude is making and a list of citations that support the claim.

    * Citations reference specific locations in source documents. The format of these citations are dependent on the type of document being cited from.

      * **For PDFs:** Citations include the page number range (1-indexed).
      * **For plain text documents:** Citations include the character index range (0-indexed).
      * **For custom content documents:** Citations include the content block index range (0-indexed) corresponding to the original content list provided.

    * Document indices are provided to indicate the reference source and are 0-indexed according to the list of all documents in your original request.
  </Step>
</Steps>

<Tip>
  **Automatic chunking vs custom content**

  By default, plain text and PDF documents are automatically chunked into sentences. If you need more control over citation granularity (for example, for bullet points or transcripts), use custom content documents instead. See [Document types](#document-types) for more details.

  For example, if you want Claude to be able to cite specific sentences from your RAG chunks, you should put each RAG chunk into a plain text document. Otherwise, if you do not want any further chunking to be done, or if you want to customize any additional chunking, you can put RAG chunks into custom content document(s).
</Tip>

### Citable vs non-citable content

* Text found within a document's `source` content can be cited from.
* `title` and `context` are optional fields that are passed to the model but not used toward cited content.
* `title` is limited in length, so the `context` field is useful for storing document metadata as text or stringified JSON.

### Citation indices

* Document indices are 0-indexed from the list of all document content blocks in the request (spanning across all messages).
* Character indices are 0-indexed with exclusive end indices.
* Page numbers are 1-indexed with exclusive end page numbers.
* Content block indices are 0-indexed with exclusive end indices from the `content` list provided in the custom content document.

### Token costs

* Enabling citations incurs a slight increase in input tokens because of system prompt additions and document chunking.
* However, the citations feature is very efficient with output tokens. Under the hood, the model is outputting citations in a standardized format that are then parsed into cited text and document location indices. The `cited_text` field is provided for convenience and does not count toward output tokens.
* When passed back in subsequent conversation turns, `cited_text` is also not counted toward input tokens.

### Feature compatibility

Citations work in conjunction with other API features including [prompt caching](/docs/en/build-with-claude/prompt-caching), [token counting](/docs/en/build-with-claude/token-counting), and [batch processing](/docs/en/build-with-claude/batch-processing).

<Warning>
  **Citations and structured outputs are incompatible**

  Citations cannot be used together with [structured outputs](/docs/en/build-with-claude/structured-outputs). If you enable citations on any user-provided document (`document` blocks or `search_result` blocks) and also include the `output_config.format` parameter (or the deprecated `output_format` parameter), the API returns a 400 error.

  This is because citations require interleaving citation blocks with text output, which is incompatible with the strict JSON schema constraints of structured outputs.
</Warning>

#### Using prompt caching with citations

Citations and prompt caching can be used together effectively.

The citation blocks generated in responses cannot be cached directly, but the source documents they reference can be cached. To optimize performance, apply `cache_control` to your top-level document content blocks.

<CodeGroup>
  ```bash cURL
  curl https://api.anthropic.com/v1/messages \
    -H "content-type: application/json" \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -d '{
      "model": "claude-opus-4-8",
      "max_tokens": 1024,
      "messages": [
        {
          "role": "user",
          "content": [
            {
              "type": "document",
              "source": {
                "type": "text",
                "media_type": "text/plain",
                "data": "This is a very long document with thousands of words..."
              },
              "citations": {"enabled": true},
              "cache_control": {"type": "ephemeral"}
            },
            {
              "type": "text",
              "text": "What does this document say about API features?"
            }
          ]
        }
      ]
    }'
  ```

  ```bash CLI
  ant messages create \
    --model claude-opus-4-8 \
    --max-tokens 1024 <<'YAML'
  messages:
    - role: user
      content:
        - type: document
          source:
            type: text
            media_type: text/plain
            data: This is a very long document with thousands of words...
          citations:
            enabled: true
          cache_control:
            type: ephemeral
        - type: text
          text: What does this document say about API features?
  YAML
  ```

  ```python Python
  client = anthropic.Anthropic()

  # Long document content (for example, technical documentation)
  long_document = (
      "This is a very long document with thousands of words..." + " ... " * 1000
  )  # Minimum cacheable length

  response = client.messages.create(
      model="claude-opus-4-8",
      max_tokens=1024,
      messages=[
          {
              "role": "user",
              "content": [
                  {
                      "type": "document",
                      "source": {
                          "type": "text",
                          "media_type": "text/plain",
                          "data": long_document,
                      },
                      "citations": {"enabled": True},
                      "cache_control": {
                          "type": "ephemeral"
                      },  # Cache the document content
                  },
                  {
                      "type": "text",
                      "text": "What does this document say about API features?",
                  },
              ],
          }
      ],
  )
  print(response)
  ```

  ```typescript TypeScript
  const client = new Anthropic();

  // Long document content (for example, technical documentation)
  const longDocument =
    "This is a very long document with thousands of words..." + " ... ".repeat(1000); // Minimum cacheable length

  const response = await client.messages.create({
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "document",
            source: {
              type: "text",
              media_type: "text/plain",
              data: longDocument
            },
            citations: { enabled: true },
            cache_control: { type: "ephemeral" } // Cache the document content
          },
          {
            type: "text",
            text: "What does this document say about API features?"
          }
        ]
      }
    ]
  });
  console.log(response);
  ```

  ```csharp C#
  var client = new AnthropicClient();

  // Long document content (for example, technical documentation)
  var longDocument =
      "This is a very long document with thousands of words..."
      + string.Concat(Enumerable.Repeat(" ... ", 1000)); // Minimum cacheable length

  var response = await client.Messages.Create(
      new()
      {
          Model = Model.ClaudeOpus4_8,
          MaxTokens = 1024,
          Messages =
          [
              new()
              {
                  Role = Role.User,
                  Content = new MessageParamContent(new List<ContentBlockParam>
                  {
                      new ContentBlockParam(new DocumentBlockParam(
                          new DocumentBlockParamSource(new PlainTextSource() { Data = longDocument })
                      )
                      {
                          Citations = new CitationsConfigParam { Enabled = true },
                          CacheControl = new CacheControlEphemeral(), // Cache the document content
                      }),
                      new ContentBlockParam(new TextBlockParam("What does this document say about API features?")),
                  }),
              },
          ],
      }
  );

  Console.WriteLine(response);
  ```

  ```go Go
  client := anthropic.NewClient()

  // Long document content (for example, technical documentation)
  longDocument := "This is a very long document with thousands of words..." +
  	strings.Repeat(" ... ", 1000) // Minimum cacheable length

  response, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
  	Model:     anthropic.ModelClaudeOpus4_8,
  	MaxTokens: 1024,
  	Messages: []anthropic.MessageParam{
  		anthropic.NewUserMessage(
  			anthropic.ContentBlockParamUnion{
  				OfDocument: &anthropic.DocumentBlockParam{
  					Source: anthropic.DocumentBlockParamSourceUnion{
  						OfText: &anthropic.PlainTextSourceParam{Data: longDocument},
  					},
  					Citations:    anthropic.CitationsConfigParam{Enabled: anthropic.Bool(true)},
  					CacheControl: anthropic.NewCacheControlEphemeralParam(), // Cache the document content
  				},
  			},
  			anthropic.NewTextBlock("What does this document say about API features?"),
  		),
  	},
  })
  if err != nil {
  	log.Fatal(err)
  }
  fmt.Println(response)
  ```

  ```java Java
  AnthropicClient client = AnthropicOkHttpClient.fromEnv();

  // Long document content (for example, technical documentation)
  String longDocument =
    "This is a very long document with thousands of words..."
      + " ... ".repeat(1000); // Minimum cacheable length

  DocumentBlockParam documentParam = DocumentBlockParam.builder()
    .source(PlainTextSource.builder().data(longDocument).build())
    .citations(CitationsConfigParam.builder().enabled(true).build())
    .cacheControl(CacheControlEphemeral.builder().build()) // Cache the document content
    .build();

  TextBlockParam textBlockParam = TextBlockParam.builder()
    .text("What does this document say about API features?")
    .build();

  MessageCreateParams params = MessageCreateParams.builder()
    .model(Model.CLAUDE_OPUS_4_8)
    .maxTokens(1024)
    .addUserMessageOfBlockParams(
      List.of(
        ContentBlockParam.ofDocument(documentParam),
        ContentBlockParam.ofText(textBlockParam)
      )
    )
    .build();

  Message message = client.messages().create(params);
  System.out.println(message);
  ```

  ```php PHP
  $client = new Client();

  // Long document content (for example, technical documentation)
  $longDocument =
      'This is a very long document with thousands of words...'
      . str_repeat(' ... ', 1000); // Minimum cacheable length

  $response = $client->messages->create(
      maxTokens: 1024,
      messages: [
          [
              'role' => 'user',
              'content' => [
                  [
                      'type' => 'document',
                      'source' => [
                          'type' => 'text',
                          'media_type' => 'text/plain',
                          'data' => $longDocument,
                      ],
                      'citations' => ['enabled' => true],
                      'cache_control' => ['type' => 'ephemeral'], // Cache the document content
                  ],
                  [
                      'type' => 'text',
                      'text' => 'What does this document say about API features?',
                  ],
              ],
          ],
      ],
      model: 'claude-opus-4-8',
  );

  echo json_encode($response, JSON_PRETTY_PRINT);
  ```

  ```ruby Ruby
  client = Anthropic::Client.new

  # Long document content (for example, technical documentation)
  long_document =
    "This is a very long document with thousands of words..." +
    " ... " * 1000 # Minimum cacheable length

  response = client.messages.create(
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "document",
            source: {
              type: "text",
              media_type: "text/plain",
              data: long_document
            },
            citations: { enabled: true },
            cache_control: { type: "ephemeral" } # Cache the document content
          },
          {
            type: "text",
            text: "What does this document say about API features?"
          }
        ]
      }
    ]
  )

  puts response
  ```
</CodeGroup>

In this example:

* The document content is cached using `cache_control` on the document block.
* Citations are enabled on the document.
* Claude can generate responses with citations while benefiting from cached document content.
* Subsequent requests using the same document benefit from the cached content.

## Document types

### Choosing a document type

Three document types are supported for citations. Documents can be provided directly in the message (base64, text, or URL) or uploaded through the [Files API](/docs/en/build-with-claude/files) and referenced by `file_id`:

| Type           | Best for                                                        | Chunking               | Citation format               |
| -------------- | --------------------------------------------------------------- | ---------------------- | ----------------------------- |
| Plain text     | Simple text documents, prose                                    | Sentence               | Character indices (0-indexed) |
| PDF            | PDF files with text content                                     | Sentence               | Page numbers (1-indexed)      |
| Custom content | Lists, transcripts, special formatting, more granular citations | No additional chunking | Block indices (0-indexed)     |

<Note>
  .csv, .xlsx, .docx, .md, and .txt files are not supported as document blocks. Convert these to plain text and include directly in message content. See [Working with other file formats](/docs/en/build-with-claude/files#working-with-other-file-formats).
</Note>

### Plain text documents

Plain text documents are automatically chunked into sentences. You can provide them inline or by reference with their `file_id`:

<Tabs>
  <Tab title="Inline text">
    The intro example at the top of this page shows a complete plain text request in every SDK. The document block uses a `text` source:

    ```json
    {
      "type": "document",
      "source": {
        "type": "text",
        "media_type": "text/plain",
        "data": "Plain text content..."
      },
      "title": "Document Title",
      "context": "Context about the document that will not be cited from",
      "citations": { "enabled": true }
    }
    ```
  </Tab>

  <Tab title="Files API">
    <Note>
      Files API document sources are in beta. These examples use the beta client path; see [Files API](/docs/en/build-with-claude/files) for upload details.
    </Note>

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
                "type": "document",
                "source": {"type": "file", "file_id": "$FILE_ID"},
                "title": "Document Title",
                "context": "Context about the document that will not be cited from",
                "citations": {"enabled": true}
              },
              {
                "type": "text",
                "text": "Summarize this document."
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
            - type: document
              source:
                type: file
                file_id: $FILE_ID
              title: Document Title
              context: Context about the document that will not be cited from
              citations:
                enabled: true
            - type: text
              text: Summarize this document.
      YAML
      ```

      ```python Python
      cited_response = client.beta.messages.create(
          model="claude-opus-4-8",
          max_tokens=1024,
          messages=[
              {
                  "role": "user",
                  "content": [
                      {
                          "type": "document",
                          "source": {"type": "file", "file_id": file_id},
                          "title": "Document Title",
                          "context": "Context about the document that will not be cited from",
                          "citations": {"enabled": True},
                      },
                      {"type": "text", "text": "Summarize this document."},
                  ],
              }
          ],
          betas=["files-api-2025-04-14"],
      )
      print(cited_response)
      ```

      ```typescript TypeScript
      const citedResponse = await client.beta.messages.create({
        model: "claude-opus-4-8",
        max_tokens: 1024,
        messages: [
          {
            role: "user",
            content: [
              {
                type: "document",
                source: { type: "file", file_id: uploaded.id },
                title: "Document Title",
                context: "Context about the document that will not be cited from",
                citations: { enabled: true },
              },
              {
                type: "text",
                text: "Summarize this document.",
              },
            ],
          },
        ],
        betas: ["files-api-2025-04-14"],
      });
      console.log(citedResponse);
      ```

      ```csharp C#
      var citedResponse = await client.Beta.Messages.Create(
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
                          new BetaRequestDocumentBlock
                          {
                              Source = new BetaFileDocumentSource { FileID = fileId },
                              Title = "Document Title",
                              Context = "Context about the document that will not be cited from",
                              Citations = new BetaCitationsConfigParam { Enabled = true },
                          },
                          new BetaTextBlockParam { Text = "Summarize this document." },
                      }
                  }
              ]
          });

      Console.WriteLine(citedResponse);
      ```

      ```go Go
      citedMsg, err := client.Beta.Messages.New(context.Background(),
      	anthropic.BetaMessageNewParams{
      		Model:     anthropic.ModelClaudeOpus4_8,
      		MaxTokens: 1024,
      		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaFilesAPI2025_04_14},
      		Messages: []anthropic.BetaMessageParam{
      			anthropic.NewBetaUserMessage(
      				anthropic.BetaContentBlockParamUnion{
      					OfDocument: &anthropic.BetaRequestDocumentBlockParam{
      						Source: anthropic.BetaRequestDocumentBlockSourceUnionParam{
      							OfFile: &anthropic.BetaFileDocumentSourceParam{FileID: fileID},
      						},
      						Title:     anthropic.String("Document Title"),
      						Context:   anthropic.String("Context about the document that will not be cited from"),
      						Citations: anthropic.BetaCitationsConfigParam{Enabled: anthropic.Bool(true)},
      					},
      				},
      				anthropic.NewBetaTextBlock("Summarize this document."),
      			),
      		},
      	})
      if err != nil {
      	log.Fatal(err)
      }
      fmt.Println(citedMsg)
      ```

      ```java Java
      MessageCreateParams citedParams = MessageCreateParams.builder()
          .model(Model.CLAUDE_OPUS_4_8)
          .addBeta("files-api-2025-04-14")
          .maxTokens(1024)
          .addUserMessageOfBetaContentBlockParams(List.of(
              BetaContentBlockParam.ofDocument(BetaRequestDocumentBlock.builder()
                  .source(BetaFileDocumentSource.builder().fileId(fileId).build())
                  .title("Document Title")
                  .context("Context about the document that will not be cited from")
                  .citations(BetaCitationsConfigParam.builder().enabled(true).build())
                  .build()),
              BetaContentBlockParam.ofText(BetaTextBlockParam.builder()
                  .text("Summarize this document.")
                  .build())
          ))
          .build();

      BetaMessage citedMessage = client.beta().messages().create(citedParams);
      System.out.println(citedMessage);
      ```

      ```php PHP
      $citedResponse = $client->beta->messages->create(
          maxTokens: 1024,
          messages: [
              [
                  'role' => 'user',
                  'content' => [
                      [
                          'type' => 'document',
                          'source' => ['type' => 'file', 'file_id' => $fileId],
                          'title' => 'Document Title',
                          'context' => 'Context about the document that will not be cited from',
                          'citations' => ['enabled' => true],
                      ],
                      ['type' => 'text', 'text' => 'Summarize this document.'],
                  ],
              ],
          ],
          model: 'claude-opus-4-8',
          betas: ['files-api-2025-04-14'],
      );

      print_r($citedResponse);
      ```

      ```ruby Ruby
      cited_response = client.beta.messages.create(
        model: "claude-opus-4-8",
        max_tokens: 1024,
        betas: ["files-api-2025-04-14"],
        messages: [
          {
            role: "user",
            content: [
              {
                type: "document",
                source: { type: "file", file_id: file_id },
                title: "Document Title",
                context: "Context about the document that will not be cited from",
                citations: { enabled: true }
              },
              {
                type: "text",
                text: "Summarize this document."
              }
            ]
          }
        ]
      )

      puts cited_response
      ```
    </CodeGroup>
  </Tab>
</Tabs>

<Accordion title="Example plain text citation">
  ```python
  {
      "type": "char_location",
      "cited_text": "The exact text being cited",  # not counted toward output tokens
      "document_index": 0,
      "document_title": "Document Title",
      "start_char_index": 0,  # 0-indexed
      "end_char_index": 50,  # exclusive
  }
  ```
</Accordion>

### PDF documents

PDF documents can be provided as base64-encoded data, a URL, or by `file_id`. PDF text is extracted and chunked into sentences. As image citations are not yet supported, PDFs that are scans of documents and do not contain extractable text will not be citable.

<Tabs>
  <Tab title="Base64">
    <CodeGroup>
      ```bash cURL
      PDF_BASE64=$(base64 /path/to/document.pdf | tr -d '\n')

      curl https://api.anthropic.com/v1/messages \
        -H "content-type: application/json" \
        -H "x-api-key: $ANTHROPIC_API_KEY" \
        -H "anthropic-version: 2023-06-01" \
        -d '{
          "model": "claude-opus-4-8",
          "max_tokens": 1024,
          "messages": [
            {
              "role": "user",
              "content": [
                {
                  "type": "document",
                  "source": {
                    "type": "base64",
                    "media_type": "application/pdf",
                    "data": "'"$PDF_BASE64"'"
                  },
                  "title": "Document Title",
                  "context": "Context about the document that will not be cited from",
                  "citations": {"enabled": true}
                },
                {
                  "type": "text",
                  "text": "Summarize this document."
                }
              ]
            }
          ]
        }'
      ```

      ```bash CLI
      ant messages create <<'YAML'
      model: claude-opus-4-8
      max_tokens: 1024
      messages:
        - role: user
          content:
            - type: document
              source:
                type: base64
                media_type: application/pdf
                data: "@/path/to/document.pdf"
              title: Document Title
              context: Context about the document that will not be cited from
              citations:
                enabled: true
            - type: text
              text: Summarize this document.
      YAML
      ```

      ```python Python
      client = anthropic.Anthropic()

      pdf_base64 = base64.standard_b64encode(
          pathlib.Path("/path/to/document.pdf").read_bytes()
      ).decode()

      response = client.messages.create(
          model="claude-opus-4-8",
          max_tokens=1024,
          messages=[
              {
                  "role": "user",
                  "content": [
                      {
                          "type": "document",
                          "source": {
                              "type": "base64",
                              "media_type": "application/pdf",
                              "data": pdf_base64,
                          },
                          "title": "Document Title",
                          "context": "Context about the document that will not be cited from",
                          "citations": {"enabled": True},
                      },
                      {"type": "text", "text": "Summarize this document."},
                  ],
              }
          ],
      )
      print(response)
      ```

      ```typescript TypeScript
      const client = new Anthropic();

      const pdfBase64 = Buffer.from(await readFile("/path/to/document.pdf")).toString("base64");

      const response = await client.messages.create({
        model: "claude-opus-4-8",
        max_tokens: 1024,
        messages: [
          {
            role: "user",
            content: [
              {
                type: "document",
                source: {
                  type: "base64",
                  media_type: "application/pdf",
                  data: pdfBase64
                },
                title: "Document Title",
                context: "Context about the document that will not be cited from",
                citations: { enabled: true }
              },
              {
                type: "text",
                text: "Summarize this document."
              }
            ]
          }
        ]
      });
      console.log(response);
      ```

      ```csharp C#
      var client = new AnthropicClient();

      var pdfBase64 = Convert.ToBase64String(await File.ReadAllBytesAsync("/path/to/document.pdf"));

      var response = await client.Messages.Create(
          new()
          {
              Model = Model.ClaudeOpus4_8,
              MaxTokens = 1024,
              Messages =
              [
                  new()
                  {
                      Role = Role.User,
                      Content = new MessageParamContent(new List<ContentBlockParam>
                      {
                          new ContentBlockParam(new DocumentBlockParam(
                              new DocumentBlockParamSource(new Base64PdfSource() { Data = pdfBase64 })
                          )
                          {
                              Title = "Document Title",
                              Context = "Context about the document that will not be cited from",
                              Citations = new CitationsConfigParam { Enabled = true },
                          }),
                          new ContentBlockParam(new TextBlockParam("Summarize this document.")),
                      }),
                  },
              ],
          }
      );

      Console.WriteLine(response);
      ```

      ```go Go
      client := anthropic.NewClient()

      pdfBytes, err := os.ReadFile("/path/to/document.pdf")
      if err != nil {
      	log.Fatal(err)
      }
      pdfBase64 := base64.StdEncoding.EncodeToString(pdfBytes)

      response, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
      	Model:     anthropic.ModelClaudeOpus4_8,
      	MaxTokens: 1024,
      	Messages: []anthropic.MessageParam{
      		anthropic.NewUserMessage(
      			anthropic.ContentBlockParamUnion{
      				OfDocument: &anthropic.DocumentBlockParam{
      					Source: anthropic.DocumentBlockParamSourceUnion{
      						OfBase64: &anthropic.Base64PDFSourceParam{
      							Data: pdfBase64,
      						},
      					},
      					Title:     anthropic.String("Document Title"),
      					Context:   anthropic.String("Context about the document that will not be cited from"),
      					Citations: anthropic.CitationsConfigParam{Enabled: anthropic.Bool(true)},
      				},
      			},
      			anthropic.NewTextBlock("Summarize this document."),
      		),
      	},
      })
      if err != nil {
      	log.Fatal(err)
      }
      fmt.Println(response)
      ```

      ```java Java
      AnthropicClient client = AnthropicOkHttpClient.fromEnv();

      byte[] pdfBytes = Files.readAllBytes(Path.of("/path/to/document.pdf"));
      String pdfBase64 = Base64.getEncoder().encodeToString(pdfBytes);

      DocumentBlockParam documentParam = DocumentBlockParam.builder()
        .source(Base64PdfSource.builder().data(pdfBase64).build())
        .title("Document Title")
        .context("Context about the document that will not be cited from")
        .citations(CitationsConfigParam.builder().enabled(true).build())
        .build();

      MessageCreateParams params = MessageCreateParams.builder()
        .model(Model.CLAUDE_OPUS_4_8)
        .maxTokens(1024)
        .addUserMessageOfBlockParams(
          List.of(
            ContentBlockParam.ofDocument(documentParam),
            ContentBlockParam.ofText(TextBlockParam.builder().text("Summarize this document.").build())
          )
        )
        .build();

      Message message = client.messages().create(params);
      System.out.println(message);
      ```

      ```php PHP
      $client = new Client();

      $pdfBase64 = base64_encode(file_get_contents('/path/to/document.pdf'));

      $response = $client->messages->create(
          maxTokens: 1024,
          messages: [
              [
                  'role' => 'user',
                  'content' => [
                      [
                          'type' => 'document',
                          'source' => [
                              'type' => 'base64',
                              'media_type' => 'application/pdf',
                              'data' => $pdfBase64,
                          ],
                          'title' => 'Document Title',
                          'context' => 'Context about the document that will not be cited from',
                          'citations' => ['enabled' => true],
                      ],
                      [
                          'type' => 'text',
                          'text' => 'Summarize this document.',
                      ],
                  ],
              ],
          ],
          model: 'claude-opus-4-8',
      );

      echo json_encode($response, JSON_PRETTY_PRINT);
      ```

      ```ruby Ruby
      client = Anthropic::Client.new

      pdf_base64 = Base64.strict_encode64(File.binread("/path/to/document.pdf"))

      response = client.messages.create(
        model: "claude-opus-4-8",
        max_tokens: 1024,
        messages: [
          {
            role: "user",
            content: [
              {
                type: "document",
                source: {
                  type: "base64",
                  media_type: "application/pdf",
                  data: pdf_base64
                },
                title: "Document Title",
                context: "Context about the document that will not be cited from",
                citations: { enabled: true }
              },
              {
                type: "text",
                text: "Summarize this document."
              }
            ]
          }
        ]
      )

      puts response
      ```
    </CodeGroup>
  </Tab>

  <Tab title="URL">
    <CodeGroup>
      ```bash cURL
      curl https://api.anthropic.com/v1/messages \
        -H "content-type: application/json" \
        -H "x-api-key: $ANTHROPIC_API_KEY" \
        -H "anthropic-version: 2023-06-01" \
        -d '{
          "model": "claude-opus-4-8",
          "max_tokens": 1024,
          "messages": [
            {
              "role": "user",
              "content": [
                {
                  "type": "document",
                  "source": {
                    "type": "url",
                    "url": "https://assets.anthropic.com/m/1cd9d098ac3e6467/original/Claude-3-Model-Card-October-Addendum.pdf"
                  },
                  "title": "Document Title",
                  "context": "Context about the document that will not be cited from",
                  "citations": {"enabled": true}
                },
                {
                  "type": "text",
                  "text": "Summarize this document."
                }
              ]
            }
          ]
        }'
      ```

      ```bash CLI
      ant messages create <<'YAML'
      model: claude-opus-4-8
      max_tokens: 1024
      messages:
        - role: user
          content:
            - type: document
              source:
                type: url
                url: https://assets.anthropic.com/m/1cd9d098ac3e6467/original/Claude-3-Model-Card-October-Addendum.pdf
              title: Document Title
              context: Context about the document that will not be cited from
              citations:
                enabled: true
            - type: text
              text: Summarize this document.
      YAML
      ```

      ```python Python
      client = anthropic.Anthropic()

      response = client.messages.create(
          model="claude-opus-4-8",
          max_tokens=1024,
          messages=[
              {
                  "role": "user",
                  "content": [
                      {
                          "type": "document",
                          "source": {
                              "type": "url",
                              "url": "https://assets.anthropic.com/m/1cd9d098ac3e6467/original/Claude-3-Model-Card-October-Addendum.pdf",
                          },
                          "title": "Document Title",
                          "context": "Context about the document that will not be cited from",
                          "citations": {"enabled": True},
                      },
                      {"type": "text", "text": "Summarize this document."},
                  ],
              }
          ],
      )
      print(response)
      ```

      ```typescript TypeScript
      const client = new Anthropic();

      const response = await client.messages.create({
        model: "claude-opus-4-8",
        max_tokens: 1024,
        messages: [
          {
            role: "user",
            content: [
              {
                type: "document",
                source: {
                  type: "url",
                  url: "https://assets.anthropic.com/m/1cd9d098ac3e6467/original/Claude-3-Model-Card-October-Addendum.pdf"
                },
                title: "Document Title",
                context: "Context about the document that will not be cited from",
                citations: { enabled: true }
              },
              {
                type: "text",
                text: "Summarize this document."
              }
            ]
          }
        ]
      });
      console.log(response);
      ```

      ```csharp C#
      var client = new AnthropicClient();

      var response = await client.Messages.Create(
          new()
          {
              Model = Model.ClaudeOpus4_8,
              MaxTokens = 1024,
              Messages =
              [
                  new()
                  {
                      Role = Role.User,
                      Content = new MessageParamContent(new List<ContentBlockParam>
                      {
                          new ContentBlockParam(new DocumentBlockParam(
                              new DocumentBlockParamSource(new UrlPdfSource()
                              {
                                  Url = "https://assets.anthropic.com/m/1cd9d098ac3e6467/original/Claude-3-Model-Card-October-Addendum.pdf",
                              })
                          )
                          {
                              Title = "Document Title",
                              Context = "Context about the document that will not be cited from",
                              Citations = new CitationsConfigParam { Enabled = true },
                          }),
                          new ContentBlockParam(new TextBlockParam("Summarize this document.")),
                      }),
                  },
              ],
          }
      );

      Console.WriteLine(response);
      ```

      ```go Go
      client := anthropic.NewClient()

      response, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
      	Model:     anthropic.ModelClaudeOpus4_8,
      	MaxTokens: 1024,
      	Messages: []anthropic.MessageParam{
      		anthropic.NewUserMessage(
      			anthropic.ContentBlockParamUnion{
      				OfDocument: &anthropic.DocumentBlockParam{
      					Source: anthropic.DocumentBlockParamSourceUnion{
      						OfURL: &anthropic.URLPDFSourceParam{
      							URL: "https://assets.anthropic.com/m/1cd9d098ac3e6467/original/Claude-3-Model-Card-October-Addendum.pdf",
      						},
      					},
      					Title:     anthropic.String("Document Title"),
      					Context:   anthropic.String("Context about the document that will not be cited from"),
      					Citations: anthropic.CitationsConfigParam{Enabled: anthropic.Bool(true)},
      				},
      			},
      			anthropic.NewTextBlock("Summarize this document."),
      		),
      	},
      })
      if err != nil {
      	log.Fatal(err)
      }
      fmt.Println(response)
      ```

      ```java Java
      AnthropicClient client = AnthropicOkHttpClient.fromEnv();

      DocumentBlockParam documentParam = DocumentBlockParam.builder()
        .source(UrlPdfSource.builder()
          .url("https://assets.anthropic.com/m/1cd9d098ac3e6467/original/Claude-3-Model-Card-October-Addendum.pdf")
          .build())
        .title("Document Title")
        .context("Context about the document that will not be cited from")
        .citations(CitationsConfigParam.builder().enabled(true).build())
        .build();

      MessageCreateParams params = MessageCreateParams.builder()
        .model(Model.CLAUDE_OPUS_4_8)
        .maxTokens(1024)
        .addUserMessageOfBlockParams(
          List.of(
            ContentBlockParam.ofDocument(documentParam),
            ContentBlockParam.ofText(TextBlockParam.builder().text("Summarize this document.").build())
          )
        )
        .build();

      Message message = client.messages().create(params);
      System.out.println(message);
      ```

      ```php PHP
      $client = new Client();

      $response = $client->messages->create(
          maxTokens: 1024,
          messages: [
              [
                  'role' => 'user',
                  'content' => [
                      [
                          'type' => 'document',
                          'source' => [
                              'type' => 'url',
                              'url' => 'https://assets.anthropic.com/m/1cd9d098ac3e6467/original/Claude-3-Model-Card-October-Addendum.pdf',
                          ],
                          'title' => 'Document Title',
                          'context' => 'Context about the document that will not be cited from',
                          'citations' => ['enabled' => true],
                      ],
                      [
                          'type' => 'text',
                          'text' => 'Summarize this document.',
                      ],
                  ],
              ],
          ],
          model: 'claude-opus-4-8',
      );

      echo json_encode($response, JSON_PRETTY_PRINT);
      ```

      ```ruby Ruby
      client = Anthropic::Client.new

      response = client.messages.create(
        model: "claude-opus-4-8",
        max_tokens: 1024,
        messages: [
          {
            role: "user",
            content: [
              {
                type: "document",
                source: {
                  type: "url",
                  url: "https://assets.anthropic.com/m/1cd9d098ac3e6467/original/Claude-3-Model-Card-October-Addendum.pdf"
                },
                title: "Document Title",
                context: "Context about the document that will not be cited from",
                citations: { enabled: true }
              },
              {
                type: "text",
                text: "Summarize this document."
              }
            ]
          }
        ]
      )

      puts response
      ```
    </CodeGroup>
  </Tab>

  <Tab title="Files API">
    <Note>
      Files API document sources are in beta. These examples use the beta client path; see [Files API](/docs/en/build-with-claude/files) for upload details.
    </Note>

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
                "type": "document",
                "source": {"type": "file", "file_id": "$FILE_ID"},
                "title": "Document Title",
                "context": "Context about the document that will not be cited from",
                "citations": {"enabled": true}
              },
              {
                "type": "text",
                "text": "Summarize this document."
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
            - type: document
              source:
                type: file
                file_id: $FILE_ID
              title: Document Title
              context: Context about the document that will not be cited from
              citations:
                enabled: true
            - type: text
              text: Summarize this document.
      YAML
      ```

      ```python Python
      cited_response = client.beta.messages.create(
          model="claude-opus-4-8",
          max_tokens=1024,
          messages=[
              {
                  "role": "user",
                  "content": [
                      {
                          "type": "document",
                          "source": {"type": "file", "file_id": file_id},
                          "title": "Document Title",
                          "context": "Context about the document that will not be cited from",
                          "citations": {"enabled": True},
                      },
                      {"type": "text", "text": "Summarize this document."},
                  ],
              }
          ],
          betas=["files-api-2025-04-14"],
      )
      print(cited_response)
      ```

      ```typescript TypeScript
      const citedResponse = await client.beta.messages.create({
        model: "claude-opus-4-8",
        max_tokens: 1024,
        messages: [
          {
            role: "user",
            content: [
              {
                type: "document",
                source: { type: "file", file_id: uploaded.id },
                title: "Document Title",
                context: "Context about the document that will not be cited from",
                citations: { enabled: true },
              },
              {
                type: "text",
                text: "Summarize this document.",
              },
            ],
          },
        ],
        betas: ["files-api-2025-04-14"],
      });
      console.log(citedResponse);
      ```

      ```csharp C#
      var citedResponse = await client.Beta.Messages.Create(
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
                          new BetaRequestDocumentBlock
                          {
                              Source = new BetaFileDocumentSource { FileID = fileId },
                              Title = "Document Title",
                              Context = "Context about the document that will not be cited from",
                              Citations = new BetaCitationsConfigParam { Enabled = true },
                          },
                          new BetaTextBlockParam { Text = "Summarize this document." },
                      }
                  }
              ]
          });

      Console.WriteLine(citedResponse);
      ```

      ```go Go
      citedMsg, err := client.Beta.Messages.New(context.Background(),
      	anthropic.BetaMessageNewParams{
      		Model:     anthropic.ModelClaudeOpus4_8,
      		MaxTokens: 1024,
      		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaFilesAPI2025_04_14},
      		Messages: []anthropic.BetaMessageParam{
      			anthropic.NewBetaUserMessage(
      				anthropic.BetaContentBlockParamUnion{
      					OfDocument: &anthropic.BetaRequestDocumentBlockParam{
      						Source: anthropic.BetaRequestDocumentBlockSourceUnionParam{
      							OfFile: &anthropic.BetaFileDocumentSourceParam{FileID: fileID},
      						},
      						Title:     anthropic.String("Document Title"),
      						Context:   anthropic.String("Context about the document that will not be cited from"),
      						Citations: anthropic.BetaCitationsConfigParam{Enabled: anthropic.Bool(true)},
      					},
      				},
      				anthropic.NewBetaTextBlock("Summarize this document."),
      			),
      		},
      	})
      if err != nil {
      	log.Fatal(err)
      }
      fmt.Println(citedMsg)
      ```

      ```java Java
      MessageCreateParams citedParams = MessageCreateParams.builder()
          .model(Model.CLAUDE_OPUS_4_8)
          .addBeta("files-api-2025-04-14")
          .maxTokens(1024)
          .addUserMessageOfBetaContentBlockParams(List.of(
              BetaContentBlockParam.ofDocument(BetaRequestDocumentBlock.builder()
                  .source(BetaFileDocumentSource.builder().fileId(fileId).build())
                  .title("Document Title")
                  .context("Context about the document that will not be cited from")
                  .citations(BetaCitationsConfigParam.builder().enabled(true).build())
                  .build()),
              BetaContentBlockParam.ofText(BetaTextBlockParam.builder()
                  .text("Summarize this document.")
                  .build())
          ))
          .build();

      BetaMessage citedMessage = client.beta().messages().create(citedParams);
      System.out.println(citedMessage);
      ```

      ```php PHP
      $citedResponse = $client->beta->messages->create(
          maxTokens: 1024,
          messages: [
              [
                  'role' => 'user',
                  'content' => [
                      [
                          'type' => 'document',
                          'source' => ['type' => 'file', 'file_id' => $fileId],
                          'title' => 'Document Title',
                          'context' => 'Context about the document that will not be cited from',
                          'citations' => ['enabled' => true],
                      ],
                      ['type' => 'text', 'text' => 'Summarize this document.'],
                  ],
              ],
          ],
          model: 'claude-opus-4-8',
          betas: ['files-api-2025-04-14'],
      );

      print_r($citedResponse);
      ```

      ```ruby Ruby
      cited_response = client.beta.messages.create(
        model: "claude-opus-4-8",
        max_tokens: 1024,
        betas: ["files-api-2025-04-14"],
        messages: [
          {
            role: "user",
            content: [
              {
                type: "document",
                source: { type: "file", file_id: file_id },
                title: "Document Title",
                context: "Context about the document that will not be cited from",
                citations: { enabled: true }
              },
              {
                type: "text",
                text: "Summarize this document."
              }
            ]
          }
        ]
      )

      puts cited_response
      ```
    </CodeGroup>
  </Tab>
</Tabs>

<Accordion title="Example PDF citation">
  ```python
  {
      "type": "page_location",
      "cited_text": "The exact text being cited",  # not counted toward output tokens
      "document_index": 0,
      "document_title": "Document Title",
      "start_page_number": 1,  # 1-indexed
      "end_page_number": 2,  # exclusive
  }
  ```
</Accordion>

### Custom content documents

Custom content documents give you control over citation granularity. No additional chunking is done and chunks are provided to the model according to the content blocks provided.

<CodeGroup>
  ```bash cURL
  curl https://api.anthropic.com/v1/messages \
    -H "content-type: application/json" \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -d '{
      "model": "claude-opus-4-8",
      "max_tokens": 1024,
      "messages": [
        {
          "role": "user",
          "content": [
            {
              "type": "document",
              "source": {
                "type": "content",
                "content": [
                  {"type": "text", "text": "First chunk"},
                  {"type": "text", "text": "Second chunk"}
                ]
              },
              "title": "Document Title",
              "context": "Context about the document that will not be cited from",
              "citations": {"enabled": true}
            },
            {
              "type": "text",
              "text": "Summarize this document."
            }
          ]
        }
      ]
    }'
  ```

  ```bash CLI
  ant messages create <<'YAML'
  model: claude-opus-4-8
  max_tokens: 1024
  messages:
    - role: user
      content:
        - type: document
          source:
            type: content
            content:
              - type: text
                text: First chunk
              - type: text
                text: Second chunk
          title: Document Title
          context: Context about the document that will not be cited from
          citations:
            enabled: true
        - type: text
          text: Summarize this document.
  YAML
  ```

  ```python Python
  client = anthropic.Anthropic()

  response = client.messages.create(
      model="claude-opus-4-8",
      max_tokens=1024,
      messages=[
          {
              "role": "user",
              "content": [
                  {
                      "type": "document",
                      "source": {
                          "type": "content",
                          "content": [
                              {"type": "text", "text": "First chunk"},
                              {"type": "text", "text": "Second chunk"},
                          ],
                      },
                      "title": "Document Title",
                      "context": "Context about the document that will not be cited from",
                      "citations": {"enabled": True},
                  },
                  {"type": "text", "text": "Summarize this document."},
              ],
          }
      ],
  )
  print(response)
  ```

  ```typescript TypeScript
  const client = new Anthropic();

  const response = await client.messages.create({
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "document",
            source: {
              type: "content",
              content: [
                { type: "text", text: "First chunk" },
                { type: "text", text: "Second chunk" }
              ]
            },
            title: "Document Title",
            context: "Context about the document that will not be cited from",
            citations: { enabled: true }
          },
          {
            type: "text",
            text: "Summarize this document."
          }
        ]
      }
    ]
  });
  console.log(response);
  ```

  ```csharp C#
  var client = new AnthropicClient();

  var response = await client.Messages.Create(
      new()
      {
          Model = Model.ClaudeOpus4_8,
          MaxTokens = 1024,
          Messages =
          [
              new()
              {
                  Role = Role.User,
                  Content = new MessageParamContent(new List<ContentBlockParam>
                  {
                      new ContentBlockParam(new DocumentBlockParam(
                          new DocumentBlockParamSource(new ContentBlockSource()
                          {
                              Content = new ContentBlockSourceContent(new List<MessageContentBlockSourceContent>
                              {
                                  new TextBlockParam("First chunk"),
                                  new TextBlockParam("Second chunk"),
                              }),
                          })
                      )
                      {
                          Title = "Document Title",
                          Context = "Context about the document that will not be cited from",
                          Citations = new CitationsConfigParam { Enabled = true },
                      }),
                      new ContentBlockParam(new TextBlockParam("Summarize this document.")),
                  }),
              },
          ],
      }
  );

  Console.WriteLine(response);
  ```

  ```go Go
  client := anthropic.NewClient()

  response, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
  	Model:     anthropic.ModelClaudeOpus4_8,
  	MaxTokens: 1024,
  	Messages: []anthropic.MessageParam{
  		anthropic.NewUserMessage(
  			anthropic.ContentBlockParamUnion{
  				OfDocument: &anthropic.DocumentBlockParam{
  					Source: anthropic.DocumentBlockParamSourceUnion{
  						OfContent: &anthropic.ContentBlockSourceParam{
  							Content: anthropic.ContentBlockSourceContentUnionParam{
  								OfContentBlockSourceContent: []anthropic.ContentBlockSourceContentItemUnionParam{
  									{OfText: &anthropic.TextBlockParam{Text: "First chunk"}},
  									{OfText: &anthropic.TextBlockParam{Text: "Second chunk"}},
  								},
  							},
  						},
  					},
  					Title:     anthropic.String("Document Title"),
  					Context:   anthropic.String("Context about the document that will not be cited from"),
  					Citations: anthropic.CitationsConfigParam{Enabled: anthropic.Bool(true)},
  				},
  			},
  			anthropic.NewTextBlock("Summarize this document."),
  		),
  	},
  })
  if err != nil {
  	log.Fatal(err)
  }
  fmt.Println(response)
  ```

  ```java Java
  AnthropicClient client = AnthropicOkHttpClient.fromEnv();

  DocumentBlockParam documentParam = DocumentBlockParam.builder()
    .source(ContentBlockSource.builder()
      .contentOfBlockSource(
        List.of(
          ContentBlockSourceContent.ofText(TextBlockParam.builder().text("First chunk").build()),
          ContentBlockSourceContent.ofText(TextBlockParam.builder().text("Second chunk").build())
        )
      )
      .build())
    .title("Document Title")
    .context("Context about the document that will not be cited from")
    .citations(CitationsConfigParam.builder().enabled(true).build())
    .build();

  MessageCreateParams params = MessageCreateParams.builder()
    .model(Model.CLAUDE_OPUS_4_8)
    .maxTokens(1024)
    .addUserMessageOfBlockParams(
      List.of(
        ContentBlockParam.ofDocument(documentParam),
        ContentBlockParam.ofText(TextBlockParam.builder().text("Summarize this document.").build())
      )
    )
    .build();

  Message message = client.messages().create(params);
  System.out.println(message);
  ```

  ```php PHP
  $client = new Client();

  $response = $client->messages->create(
      maxTokens: 1024,
      messages: [
          [
              'role' => 'user',
              'content' => [
                  [
                      'type' => 'document',
                      'source' => [
                          'type' => 'content',
                          'content' => [
                              ['type' => 'text', 'text' => 'First chunk'],
                              ['type' => 'text', 'text' => 'Second chunk'],
                          ],
                      ],
                      'title' => 'Document Title',
                      'context' => 'Context about the document that will not be cited from',
                      'citations' => ['enabled' => true],
                  ],
                  [
                      'type' => 'text',
                      'text' => 'Summarize this document.',
                  ],
              ],
          ],
      ],
      model: 'claude-opus-4-8',
  );

  echo json_encode($response, JSON_PRETTY_PRINT);
  ```

  ```ruby Ruby
  client = Anthropic::Client.new

  response = client.messages.create(
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "document",
            source: {
              type: "content",
              content: [
                { type: "text", text: "First chunk" },
                { type: "text", text: "Second chunk" }
              ]
            },
            title: "Document Title",
            context: "Context about the document that will not be cited from",
            citations: { enabled: true }
          },
          {
            type: "text",
            text: "Summarize this document."
          }
        ]
      }
    ]
  )

  puts response
  ```
</CodeGroup>

<Accordion title="Example citation">
  ```python
  {
      "type": "content_block_location",
      "cited_text": "The exact text being cited",  # not counted toward output tokens
      "document_index": 0,
      "document_title": "Document Title",
      "start_block_index": 0,  # 0-indexed
      "end_block_index": 1,  # exclusive
  }
  ```
</Accordion>

***

## Response structure

When citations are enabled, responses include multiple text blocks with citations:

```python
{
    "content": [
        {"type": "text", "text": "According to the document, "},
        {
            "type": "text",
            "text": "the grass is green",
            "citations": [
                {
                    "type": "char_location",
                    "cited_text": "The grass is green.",
                    "document_index": 0,
                    "document_title": "Example Document",
                    "start_char_index": 0,
                    "end_char_index": 20,
                }
            ],
        },
        {"type": "text", "text": " and "},
        {
            "type": "text",
            "text": "the sky is blue",
            "citations": [
                {
                    "type": "char_location",
                    "cited_text": "The sky is blue.",
                    "document_index": 0,
                    "document_title": "Example Document",
                    "start_char_index": 20,
                    "end_char_index": 36,
                }
            ],
        },
        {
            "type": "text",
            "text": ". Information from page 5 states that ",
        },
        {
            "type": "text",
            "text": "water is essential",
            "citations": [
                {
                    "type": "page_location",
                    "cited_text": "Water is essential for life.",
                    "document_index": 1,
                    "document_title": "PDF Document",
                    "start_page_number": 5,
                    "end_page_number": 6,
                }
            ],
        },
        {
            "type": "text",
            "text": ". The custom document mentions ",
        },
        {
            "type": "text",
            "text": "important findings",
            "citations": [
                {
                    "type": "content_block_location",
                    "cited_text": "These are important findings.",
                    "document_index": 2,
                    "document_title": "Custom Content Document",
                    "start_block_index": 0,
                    "end_block_index": 1,
                }
            ],
        },
    ]
}
```

### Streaming support

For streaming responses, citations arrive as a `citations_delta` delta type inside `content_block_delta` events. Each delta contains a single citation to add to the `citations` list on the current `text` content block.

<AccordionGroup>
  <Accordion title="Example streaming events">
    ```sse
    event: message_start
    data: {"type": "message_start", ...}

    event: content_block_start
    data: {"type": "content_block_start", "index": 0, ...}

    event: content_block_delta
    data: {"type": "content_block_delta", "index": 0,
           "delta": {"type": "text_delta", "text": "According to..."}}

    event: content_block_delta
    data: {"type": "content_block_delta", "index": 0,
           "delta": {"type": "citations_delta",
                     "citation": {
                         "type": "char_location",
                         "cited_text": "...",
                         "document_index": 0,
                         ...
                     }}}

    event: content_block_stop
    data: {"type": "content_block_stop", "index": 0}

    event: message_stop
    data: {"type": "message_stop"}
    ```
  </Accordion>
</AccordionGroup>

## Next steps

<CardGroup cols={2}>
  <Card title="Streaming messages" icon="wifi-high" href="/docs/en/build-with-claude/streaming">
    Handle the `citations_delta` delta type alongside text deltas to render cited responses as they stream.
  </Card>

  <Card title="Search results" icon="book-bookmark" href="/docs/en/build-with-claude/search-results">
    Pass search results from your RAG pipeline as first-class content blocks with built-in citation support.
  </Card>

  <Card title="PDF support" icon="file" href="/docs/en/build-with-claude/pdf-support">
    Learn how Claude extracts text from PDFs and how page-based citations map back to your source files.
  </Card>

  <Card title="Files API" icon="hard-drives" href="/docs/en/build-with-claude/files">
    Upload documents once and reference them by `file_id` across multiple citation requests.
  </Card>
</CardGroup>
