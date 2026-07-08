# Vision

Claude's vision capabilities allow it to understand and analyze images, opening up exciting possibilities for multimodal interaction.

---

This guide describes how to send images to Claude, the limits and costs that apply, and where to find guidance for [coordinate-based workflows](/docs/en/build-with-claude/vision-coordinates).

***

## Send images to Claude

Use Claude's vision capabilities through:

* [claude.ai](https://claude.ai/). Upload an image like you would a file, or drag and drop an image directly into the chat window.
* The [Anthropic Workbench](/workbench/). A button to add images appears at the top right of every User message block.
* API request. See the following examples.

On the API, provide images to Claude as `image` content blocks using one of three source types:

1. A base64-encoded image embedded in the request body
2. A URL reference to an image hosted online
3. A `file_id` returned by the [Files API](/docs/en/build-with-claude/files) (upload once, reference many times)

<Note>
  On Amazon Bedrock and Google Cloud, only base64-encoded sources are currently available.
</Note>

<Tip>
  Just as [placing long documents before your query](/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#long-context-prompting) improves results in text prompts, Claude works best when images come before text. Images placed after text or interpolated with text still perform well, but if your use case allows it, prefer an image-then-text structure.
</Tip>

### Base64-encoded image example

<CodeGroup>
  ```bash cURL
  curl https://api.anthropic.com/v1/messages \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
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
            "type": "image",
            "source": {
              "type": "base64",
              "media_type": "image/jpeg",
              "data": "$BASE64_IMAGE_DATA"
            }
          },
          {
            "type": "text",
            "text": "Describe this image."
          }
        ]
      }
    ]
  }
  EOF
  ```

  ```bash CLI
  curl -sSo ./image.jpg \
    https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg

  ant messages create <<'YAML'
  model: claude-opus-4-8
  max_tokens: 1024
  messages:
    - role: user
      content:
        - type: image
          source:
            type: base64
            media_type: image/jpeg
            data: "@./image.jpg"
        - type: text
          text: Describe this image.
  YAML
  ```

  ```python Python
  image1_data = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC"
  image1_media_type = "image/png"

  client = anthropic.Anthropic()
  message = client.messages.create(
      model="claude-opus-4-8",
      max_tokens=1024,
      messages=[
          {
              "role": "user",
              "content": [
                  {
                      "type": "image",
                      "source": {
                          "type": "base64",
                          "media_type": image1_media_type,
                          "data": image1_data,
                      },
                  },
                  {"type": "text", "text": "Describe this image."},
              ],
          }
      ],
  )
  print(message)
  ```

  ```typescript TypeScript
  const anthropic = new Anthropic({
    apiKey: process.env.ANTHROPIC_API_KEY
  });

  const message = await anthropic.messages.create({
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "image",
            source: {
              type: "base64",
              media_type: "image/jpeg",
              data: imageData // Base64-encoded image data as string
            }
          },
          {
            type: "text",
            text: "Describe this image."
          }
        ]
      }
    ]
  });

  console.log(message);
  ```

  ```csharp C#
  using System.Collections.Generic;
  using Anthropic;
  using Anthropic.Models.Messages;

  AnthropicClient client = new();

  string imageData = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC";

  var message = await client.Messages.Create(new MessageCreateParams
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
                  new ContentBlockParam(new ImageBlockParam(
                      new ImageBlockParamSource(new Base64ImageSource()
                      {
                          Data = imageData,
                          MediaType = MediaType.ImagePng,
                      })
                  )),
                  new ContentBlockParam(new TextBlockParam("Describe this image.")),
              }),
          }
      ]
  });

  Console.WriteLine(message);
  ```

  ```go Go
  client := anthropic.NewClient()

  imageData := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC"

  message, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
  	Model:     anthropic.ModelClaudeOpus4_8,
  	MaxTokens: 1024,
  	Messages: []anthropic.MessageParam{
  		anthropic.NewUserMessage(
  			anthropic.NewImageBlockBase64("image/png", imageData),
  			anthropic.NewTextBlock("Describe this image."),
  		),
  	},
  })
  if err != nil {
  	log.Fatal(err)
  }

  fmt.Println(message)
  ```

  ```java Java
  AnthropicClient client = AnthropicOkHttpClient.fromEnv();
  String imageData = ""; // Base64-encoded image data as string

  List<ContentBlockParam> contentBlockParams = List.of(
    ContentBlockParam.ofImage(
      ImageBlockParam.builder()
        .source(
          Base64ImageSource.builder()
            .mediaType(Base64ImageSource.MediaType.IMAGE_JPEG)
            .data(imageData)
            .build()
        )
        .build()
    ),
    ContentBlockParam.ofText(TextBlockParam.builder().text("Describe this image.").build())
  );
  Message message = client
    .messages()
    .create(
      MessageCreateParams.builder()
        .model(Model.CLAUDE_OPUS_4_8)
        .maxTokens(1024)
        .addUserMessageOfBlockParams(contentBlockParams)
        .build()
    );

  System.out.println(message);
  ```

  ```php PHP
  $client = new Client();

  $imageData = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC";

  $message = $client->messages->create(
      maxTokens: 1024,
      messages: [
          [
              'role' => 'user',
              'content' => [
                  [
                      'type' => 'image',
                      'source' => [
                          'type' => 'base64',
                          'media_type' => 'image/png',
                          'data' => $imageData,
                      ],
                  ],
                  ['type' => 'text', 'text' => 'Describe this image.'],
              ],
          ],
      ],
      model: 'claude-opus-4-8',
  );

  echo $message->content[0]->text;
  ```

  ```ruby Ruby
  client = Anthropic::Client.new

  image_data = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC"

  message = client.messages.create(
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "image",
            source: {
              type: "base64",
              media_type: "image/png",
              data: image_data
            }
          },
          { type: "text", text: "Describe this image." }
        ]
      }
    ]
  )

  puts message
  ```
</CodeGroup>

### URL-based image example

<CodeGroup>
  ```bash cURL
  curl https://api.anthropic.com/v1/messages \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -H "content-type: application/json" \
    -d '{
      "model": "claude-opus-4-8",
      "max_tokens": 1024,
      "messages": [
        {
          "role": "user",
          "content": [
            {
              "type": "image",
              "source": {
                "type": "url",
                "url": "https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg"
              }
            },
            {
              "type": "text",
              "text": "Describe this image."
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
        - type: image
          source:
            type: url
            url: https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg
        - type: text
          text: Describe this image.
  YAML
  ```

  ```python Python
  client = anthropic.Anthropic()
  message = client.messages.create(
      model="claude-opus-4-8",
      max_tokens=1024,
      messages=[
          {
              "role": "user",
              "content": [
                  {
                      "type": "image",
                      "source": {
                          "type": "url",
                          "url": "https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg",
                      },
                  },
                  {"type": "text", "text": "Describe this image."},
              ],
          }
      ],
  )
  print(message)
  ```

  ```typescript TypeScript
  const anthropic = new Anthropic({
    apiKey: process.env.ANTHROPIC_API_KEY
  });

  const message = await anthropic.messages.create({
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "image",
            source: {
              type: "url",
              url: "https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg"
            }
          },
          {
            type: "text",
            text: "Describe this image."
          }
        ]
      }
    ]
  });

  console.log(message);
  ```

  ```csharp C#
  using System.Collections.Generic;
  using Anthropic;
  using Anthropic.Models.Messages;

  AnthropicClient client = new();

  var message = await client.Messages.Create(new MessageCreateParams
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
                  new ContentBlockParam(new ImageBlockParam(
                      new ImageBlockParamSource(new UrlImageSource()
                      {
                          Url = "https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg",
                      })
                  )),
                  new ContentBlockParam(new TextBlockParam("Describe this image.")),
              }),
          }
      ]
  });

  Console.WriteLine(message);
  ```

  ```go Go
  client := anthropic.NewClient()

  message, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
  	Model:     anthropic.ModelClaudeOpus4_8,
  	MaxTokens: 1024,
  	Messages: []anthropic.MessageParam{
  		anthropic.NewUserMessage(
  			anthropic.NewImageBlock(anthropic.URLImageSourceParam{
  				URL: "https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg",
  			}),
  			anthropic.NewTextBlock("Describe this image."),
  		),
  	},
  })
  if err != nil {
  	log.Fatal(err)
  }

  fmt.Println(message)
  ```

  ```java Java
  AnthropicClient client = AnthropicOkHttpClient.fromEnv();

  List<ContentBlockParam> contentBlockParams = List.of(
    ContentBlockParam.ofImage(
      ImageBlockParam.builder()
        .source(
          UrlImageSource.builder()
            .url(
              "https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg"
            )
            .build()
        )
        .build()
    ),
    ContentBlockParam.ofText(TextBlockParam.builder().text("Describe this image.").build())
  );
  Message message = client
    .messages()
    .create(
      MessageCreateParams.builder()
        .model(Model.CLAUDE_OPUS_4_8)
        .maxTokens(1024)
        .addUserMessageOfBlockParams(contentBlockParams)
        .build()
    );
  System.out.println(message);
  ```

  ```php PHP
  $client = new Client();

  $message = $client->messages->create(
      maxTokens: 1024,
      messages: [
          [
              'role' => 'user',
              'content' => [
                  [
                      'type' => 'image',
                      'source' => [
                          'type' => 'url',
                          'url' => 'https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg',
                      ],
                  ],
                  ['type' => 'text', 'text' => 'Describe this image.'],
              ],
          ],
      ],
      model: 'claude-opus-4-8',
  );

  echo $message->content[0]->text;
  ```

  ```ruby Ruby
  client = Anthropic::Client.new

  message = client.messages.create(
    model: "claude-opus-4-8",
    max_tokens: 1024,
    messages: [
      {
        role: "user",
        content: [
          {
            type: "image",
            source: {
              type: "url",
              url: "https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg"
            }
          },
          { type: "text", text: "Describe this image." }
        ]
      }
    ]
  )

  puts message
  ```
</CodeGroup>

### Files API image example

For images you'll use repeatedly or when you want to avoid encoding overhead, use the [Files API](/docs/en/build-with-claude/files). Upload the image once, then reference the returned `file_id` in subsequent messages instead of resending base64 data.

<Tip>
  In multi-turn conversations and agentic workflows, each request resends the full conversation history. If images are base64-encoded, the full image bytes are included in the payload on every turn, which can significantly increase request size and latency as the conversation grows. Uploading images to the Files API and referencing them by `file_id` keeps request payloads small regardless of how many images accumulate in the conversation history.
</Tip>

<CodeGroup>
  ```bash cURL
  # First, upload your image to the Files API
  curl -X POST https://api.anthropic.com/v1/files \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -H "anthropic-beta: files-api-2025-04-14" \
    -F "file=@image.jpg"

  # Then use the returned file_id in your message
  curl https://api.anthropic.com/v1/messages \
    -H "x-api-key: $ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -H "anthropic-beta: files-api-2025-04-14" \
    -H "content-type: application/json" \
    -d '{
      "model": "claude-opus-4-8",
      "max_tokens": 1024,
      "messages": [
        {
          "role": "user",
          "content": [
            {
              "type": "image",
              "source": {
                "type": "file",
                "file_id": "file_abc123"
              }
            },
            {
              "type": "text",
              "text": "Describe this image."
            }
          ]
        }
      ]
    }'
  ```

  ```bash CLI
  curl -sSo image.jpg \
    https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg

  # First, upload your image to the Files API
  FILE_ID=$(ant beta:files upload \
    --file ./image.jpg \
    --transform id --raw-output)

  # Then use the returned file_id in your message
  ant beta:messages create \
    --beta files-api-2025-04-14 \
    --transform content --format yaml <<YAML
  model: claude-opus-4-8
  max_tokens: 1024
  messages:
    - role: user
      content:
        - type: image
          source:
            type: file
            file_id: $FILE_ID
        - type: text
          text: Describe this image.
  YAML
  ```

  ```python Python
  client = anthropic.Anthropic()

  # Upload the image file
  with open("image.jpg", "rb") as f:
      file_upload = client.beta.files.upload(file=("image.jpg", f, "image/jpeg"))

  # Use the uploaded file in a message
  message = client.beta.messages.create(
      model="claude-opus-4-8",
      max_tokens=1024,
      betas=["files-api-2025-04-14"],
      messages=[
          {
              "role": "user",
              "content": [
                  {
                      "type": "image",
                      "source": {"type": "file", "file_id": file_upload.id},
                  },
                  {"type": "text", "text": "Describe this image."},
              ],
          }
      ],
  )

  print(message.content)
  ```

  ```typescript TypeScript
  import Anthropic, { toFile } from "@anthropic-ai/sdk";
  import fs from "node:fs";

  const anthropic = new Anthropic();

  // Upload the image file
  const fileUpload = await anthropic.beta.files.upload({
    file: await toFile(fs.createReadStream("image.jpg"), undefined, { type: "image/jpeg" })
  });

  // Use the uploaded file in a message
  const response = await anthropic.beta.messages.create({
    model: "claude-opus-4-8",
    max_tokens: 1024,
    betas: ["files-api-2025-04-14"],
    messages: [
      {
        role: "user",
        content: [
          {
            type: "image",
            source: {
              type: "file",
              file_id: fileUpload.id
            }
          },
          {
            type: "text",
            text: "Describe this image."
          }
        ]
      }
    ]
  });

  console.log(response);
  ```

  ```csharp C#
  using Anthropic;

  var client = new AnthropicClient();

  // Upload the image file
  var fileUpload = await client.Beta.Files.Upload(
      new FileUploadParams { File = File.OpenRead("image.jpg") });

  // Use the uploaded file in a message
  var response = await client.Beta.Messages.Create(
      new MessageCreateParams
      {
          Model = "claude-opus-4-8",
          MaxTokens = 1024,
          Betas = new[] { "files-api-2025-04-14" },
          Messages = new[]
          {
              new BetaMessageParam
              {
                  Role = "user",
                  Content = new object[]
                  {
                      new
                      {
                          type = "image",
                          source = new { type = "file", file_id = fileUpload.Id }
                      },
                      new { type = "text", text = "Describe this image." }
                  }
              }
          }
      });

  Console.WriteLine(response);
  ```

  ```go Go
  client := anthropic.NewClient()

  // Upload the image file
  file, err := os.Open("image.jpg")
  if err != nil {
  	log.Fatal(err)
  }
  defer file.Close()

  fileUpload, err := client.Beta.Files.Upload(context.Background(),
  	anthropic.BetaFileUploadParams{
  		File: file,
  	})
  if err != nil {
  	log.Fatal(err)
  }

  // Use the uploaded file in a message
  message, err := client.Beta.Messages.New(context.Background(),
  	anthropic.BetaMessageNewParams{
  		Model:     anthropic.ModelClaudeOpus4_8,
  		MaxTokens: 1024,
  		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaFilesAPI2025_04_14},
  		Messages: []anthropic.BetaMessageParam{
  			anthropic.NewBetaUserMessage(
  				anthropic.NewBetaImageBlock(anthropic.BetaFileImageSourceParam{
  					FileID: fileUpload.ID,
  				}),
  				anthropic.NewBetaTextBlock("Describe this image."),
  			),
  		},
  	})
  if err != nil {
  	log.Fatal(err)
  }

  fmt.Println(message.Content)
  ```

  ```java Java
  import com.anthropic.models.beta.files.FileMetadata;
  import com.anthropic.models.beta.files.FileUploadParams;
  // ...
      AnthropicClient client = AnthropicOkHttpClient.fromEnv();

      // Upload the image file
      FileMetadata file = client
        .beta()
        .files()
        .upload(
          FileUploadParams.builder().file(Files.newInputStream(Path.of("image.jpg"))).build()
        );

      // Use the uploaded file in a message
      ImageBlockParam imageParam = ImageBlockParam.builder().fileSource(file.id()).build();

      MessageCreateParams params = MessageCreateParams.builder()
        .model(Model.CLAUDE_OPUS_4_8)
        .maxTokens(1024)
        .addUserMessageOfBlockParams(
          List.of(
            ContentBlockParam.ofImage(imageParam),
            ContentBlockParam.ofText(
              TextBlockParam.builder().text("Describe this image.").build()
            )
          )
        )
        .build();

      Message message = client.messages().create(params);
      System.out.println(message.content());
  ```

  ```php PHP
  $client = new Client();

  // Upload the image file
  $fileUpload = $client->beta->files->upload(
      file: fopen('image.jpg', 'r'),
  );

  // Use the uploaded file in a message
  $message = $client->beta->messages->create(
      maxTokens: 1024,
      messages: [
          [
              'role' => 'user',
              'content' => [
                  [
                      'type' => 'image',
                      'source' => ['type' => 'file', 'file_id' => $fileUpload->id],
                  ],
                  ['type' => 'text', 'text' => 'Describe this image.'],
              ],
          ],
      ],
      model: 'claude-opus-4-8',
      betas: ['files-api-2025-04-14'],
  );

  echo $message->content[0]->text;
  ```

  ```ruby Ruby
  client = Anthropic::Client.new

  # Upload the image file
  file_upload = client.beta.files.upload(
    file: File.open("image.jpg", "rb")
  )

  # Use the uploaded file in a message
  message = client.beta.messages.create(
    model: "claude-opus-4-8",
    max_tokens: 1024,
    betas: ["files-api-2025-04-14"],
    messages: [
      {
        role: "user",
        content: [
          {
            type: "image",
            source: { type: "file", file_id: file_upload.id }
          },
          { type: "text", text: "Describe this image." }
        ]
      }
    ]
  )

  puts message.content
  ```
</CodeGroup>

See [Messages API examples](/docs/en/api/messages/create) for more example code and parameter details.

### Multiple images

You can include multiple images in a single request, and Claude analyzes them jointly. This is useful for comparing images, asking about differences, or working with a sequence such as pages of a document. When sending several images, introduce each one with a short text label (`Image 1:`, `Image 2:`, and so on) so you can refer to them by name in your prompt and in follow-up turns.

```python Python
client = anthropic.Anthropic()
message = client.messages.create(
    model="claude-opus-4-8",
    max_tokens=1024,
    messages=[
        {
            "role": "user",
            "content": [
                {"type": "text", "text": "Image 1:"},
                {
                    "type": "image",
                    "source": {
                        "type": "base64",
                        "media_type": "image/png",
                        "data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC",
                    },
                },
                {"type": "text", "text": "Image 2:"},
                {
                    "type": "image",
                    "source": {
                        "type": "base64",
                        "media_type": "image/png",
                        "data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGNgYPgPAAEDAQAIicLsAAAAAElFTkSuQmCC",
                    },
                },
                {"type": "text", "text": "How are these images different?"},
            ],
        }
    ],
)
print(message)
```

In a multi-turn conversation, add new images in later `user` turns the same way. Claude has access to every image from earlier turns, so follow-up questions such as "Are these similar to the first two?" work without including the earlier images again in the new turn's content.

***

## Image limits and costs

### Request limits

The maximum number of images per message or request is:

* 20 per message on [claude.ai](https://claude.ai/).
* 100 per request on the API, for models with a 200k-token context window.
* 600 per request on the API, for all other models.

The maximum dimensions per image are 8000x8000 px.

If a single API request contains more than 20 images, a stricter per-image dimension limit applies. On Amazon Bedrock and Google Cloud, document blocks such as PDFs also count toward this threshold. Images exceeding the stricter limit are rejected with an `invalid_request_error` whose message references "many-image requests" and states the current limit in pixels. To stay under the limit on all platforms, either resize each image so that neither dimension exceeds 2000 px, or keep the request to 20 or fewer image and document blocks.

The maximum size per image is:

* 10 MB (base64-encoded) when using the Claude API directly.
* 5 MB (base64-encoded) on Amazon Bedrock and Google Cloud.
* 10 MB on [claude.ai](https://claude.ai/).

<Note>
  Although the API supports up to 600 images per request, [request size limits](/docs/en/api/overview#request-size-limits) (32 MB for standard endpoints; lower on some partner-operated platforms, for example, Amazon Bedrock and Google Cloud) can be reached first. For many images, consider uploading with the [Files API](#files-api-image-example) and referencing by `file_id` to keep request payloads small.

  Even when using the Files API, requests with many large images can fail before reaching the 600-image count. Reduce image dimensions or file sizes (for example, by downsampling) before uploading (see [Resolution and token cost](#evaluate-image-size)).
</Note>

### Supported formats

Claude supports JPEG, PNG, GIF, and WebP images (`image/jpeg`, `image/png`, `image/gif`, `image/webp`). Animations are unsupported, and only the first frame is used.

### Resolution and token cost

Claude views images in patches instead of pixels. Each patch is a 28×28-pixel block of the image, referred to as a visual token. An image, therefore, costs `⌈width / 28⌉ × ⌈height / 28⌉` visual tokens.

Each model has a maximum native image resolution, expressed as a long-edge limit and a visual-token limit. Images larger than either limit are downscaled before processing; see [How Claude resizes and pads images](/docs/en/build-with-claude/vision-coordinates#how-claude-resizes-and-pads-images) for the exact rule.

| Resolution tier | Models                                                                             | Max long edge | Max visual tokens |
| --------------- | ---------------------------------------------------------------------------------- | ------------- | ----------------- |
| High-resolution | Claude Fable 5, Claude Mythos 5, Claude Opus 4.8, Claude Opus 4.7, Claude Sonnet 5 | 2576 px       | 4784              |
| Standard        | All other models                                                                   | 1568 px       | 1568              |

High-resolution support is automatic on the listed models and requires no beta header or client-side opt-in.

The following table shows the visual-token cost for several image sizes on each tier:

| Image size                     | Standard-tier tokens                                                                                                                                         | High-resolution-tier tokens                                                                                                                              |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 200x200 px (0.04 megapixels)   | 64                                                                                                                                                           | 64                                                                                                                                                       |
| 1000x1000 px (1 megapixel)     | 1296                                                                                                                                                         | 1296                                                                                                                                                     |
| 1092x1092 px (1.19 megapixels) | 1521                                                                                                                                                         | 1521                                                                                                                                                     |
| 1920x1080 px (2.07 megapixels) | <Tooltip tooltipContent="Exceeds the standard tier's resolution limit; the image is downscaled before processing, which caps the token cost.">1560</Tooltip> | 2691                                                                                                                                                     |
| 2000x1500 px (3 megapixels)    | <Tooltip tooltipContent="Exceeds the standard tier's resolution limit; the image is downscaled before processing, which caps the token cost.">1564</Tooltip> | 3888                                                                                                                                                     |
| 3840x2160 px (8.29 megapixels) | <Tooltip tooltipContent="Exceeds the standard tier's resolution limit; the image is downscaled before processing, which caps the token cost.">1560</Tooltip> | <Tooltip tooltipContent="Exceeds the high-resolution tier's limit; the image is downscaled before processing, which caps the token cost.">4784</Tooltip> |

To estimate cost, multiply the token count by the [per-token price of the model](https://claude.com/pricing) you're using. For example, at Claude Haiku 4.5's $1 per million input tokens (standard tier), the 1000×1000 image costs about $1.30 per thousand images. At Claude Opus 4.8's $5 per million (high-resolution tier), the same image costs about $6.48 per thousand and the 4K image about $23.92 per thousand.

High-resolution images can use up to roughly three times more visual tokens than the same image on a standard-tier model. If you don't need the additional fidelity that high resolution provides for computer use, screenshot understanding, and dense documents, downsample images before sending to control token costs. To minimize latency and to simplify [coordinate-based workflows](/docs/en/build-with-claude/vision-coordinates), prefer resizing images before uploading them.

### Image quality guidance

When providing images to Claude, keep the following in mind for best results:

* **Image clarity:** Ensure images are clear and not too blurry or pixelated.
* **Text:** If the image contains important text, make sure it's legible and not too small. Avoid cropping out key visual context solely to enlarge the text.
* **Resizing:** Take into account that your image might be resized if it is too large (see [Resolution and token cost](#evaluate-image-size)); this might, for example, make text less legible. Consider pre-resizing your images, cropping them, or both.
* **Image compression:** Compressing images before sending them, using a lossy format such as JPEG or WebP (lossy mode), can reduce latency by reducing the size of requests. However, this can introduce artifacts that are detrimental to model performance, especially when multiple compression passes are applied. For example, heavy JPEG compression can make text difficult to read. Confirm your compression settings are appropriate for the task by inspecting the actual images sent to the API.

***

## Coordinates and bounding boxes

For bounding boxes, points, and pixel coordinates, see [Coordinates and bounding boxes](/docs/en/build-with-claude/vision-coordinates). Claude returns absolute pixel coordinates relative to the image it sees after resizing; that guide covers how Claude resizes and pads images and how to pre-resize or rescale so coordinates line up with your original image.

***

## Limitations

Although Claude's image understanding capabilities are cutting-edge, there are some limitations to be aware of:

* **People identification:** Claude [cannot be used](https://www.anthropic.com/legal/aup) to name people in images and refuses to do so.
* **Accuracy:** Claude might hallucinate or make mistakes when interpreting low-quality, rotated, or very small images under 200 pixels.
* **Spatial reasoning:** Claude's coordinate and localization outputs are approximate. Follow the guidance in [Coordinates and bounding boxes](/docs/en/build-with-claude/vision-coordinates) and verify outputs before relying on them.
* **Counting:** Claude can give approximate counts of objects in an image but might not always be precisely accurate, especially with large numbers of small objects.
* **AI-generated images:** Claude cannot determine whether an image is AI-generated and might be incorrect if asked. Do not rely on it to detect fake or synthetic images.
* **Inappropriate content:** Claude does not process inappropriate or explicit images that violate the [Acceptable Use Policy](https://www.anthropic.com/legal/aup).
* **Healthcare applications:** Although Claude can analyze general medical images, it is not designed to interpret complex diagnostic scans such as CTs or MRIs. Claude's outputs should not be considered a substitute for professional medical advice or diagnosis.

Always carefully review and verify Claude's image interpretations, especially for high-stakes use cases. Do not use Claude for tasks requiring perfect precision or sensitive image analysis without human oversight.

***

## FAQ

<AccordionGroup>
  <Accordion title="What image file types does Claude support?">
    JPEG, PNG, GIF, and WebP. See [Supported formats](#supported-formats).
  </Accordion>

  <Accordion title="Can Claude read image URLs?">
    Yes. Use the `url` source type instead of `base64` in the `image` content block. See the [URL-based image example](#url-based-image-example).
  </Accordion>

  <Accordion title="Is there a limit to the image file size I can upload?">
    Yes. See [Request limits](#request-limits) for per-image and overall request size limits across the Claude API, Amazon Bedrock, Google Cloud, and claude.ai.
  </Accordion>

  <Accordion title="How many images can I include in one request?">
    Up to 600 per API request (100 for models with a 200k-token context window) and 20 per turn on claude.ai. See [Request limits](#request-limits) for details and the lower per-image dimension limit that applies above 20 images.
  </Accordion>

  <Accordion title="Does Claude read image metadata?">
    No, Claude does not parse or receive any metadata from images passed to it.
  </Accordion>

  <Accordion title="Can I delete images I've uploaded?">
    No. Image uploads are ephemeral and not stored beyond the duration of the API request. Uploaded images are automatically deleted after they have been processed.
  </Accordion>

  <Accordion title="Where can I find details on data privacy for image uploads?">
    Refer to the Anthropic privacy policy page for information on how uploaded images and other data are handled. Anthropic does not use uploaded images to train models.
  </Accordion>

  <Accordion title="What if Claude's image interpretation seems wrong?">
    If Claude's image interpretation seems incorrect:

    1. Ensure the image is clear, high-quality, and correctly oriented.
    2. Try prompt engineering techniques to improve results.
    3. If the issue persists, flag the output in claude.ai (thumbs up/down) or contact the [support team](https://support.claude.com/).

    Your feedback helps improve Claude!
  </Accordion>

  <Accordion title="Can Claude generate or edit images?">
    No, Claude is an image understanding model only. It can interpret and analyze images, but it cannot generate, produce, edit, manipulate, or create images.
  </Accordion>
</AccordionGroup>

***

## Next steps

<CardGroup cols={2}>
  <Card title="Multimodal cookbook" icon="image" href="https://platform.claude.com/cookbook/multimodal-getting-started-with-vision">
    Get tips and best-practice techniques for tasks such as interpreting charts and extracting content from forms.
  </Card>

  <Card title="API reference" icon="code" href="/docs/en/api/messages/create">
    See the Messages API documentation, including example API calls involving images.
  </Card>
</CardGroup>
