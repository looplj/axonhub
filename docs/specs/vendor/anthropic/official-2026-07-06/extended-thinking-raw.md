# Extended thinking

Give Claude enhanced reasoning for complex tasks and control how thinking content is returned.

---

<Note>
  This feature is eligible for [Zero Data Retention (ZDR)](/docs/en/build-with-claude/api-and-data-retention). When your organization has a ZDR arrangement, data sent through this feature is not stored after the API response is returned.
</Note>

Extended thinking gives Claude enhanced reasoning capabilities for complex tasks, while providing varying levels of transparency into its step-by-step thought process before it delivers its final answer.

## Supported models

Extended thinking is available on all current Claude models. How you enable it depends on the model:

| Model                                                    | Manual extended thinking (`budget_tokens`)                             | Recommended                                                                                                                                    |
| -------------------------------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| Claude Fable 5 Claude Mythos 5                           | Not supported (400 error)                                              | [Adaptive thinking](/docs/en/build-with-claude/adaptive-thinking), always on; use [effort](/docs/en/build-with-claude/effort) to control depth |
| [Claude Mythos Preview](https://anthropic.com/glasswing) | Supported                                                              | [Adaptive thinking](/docs/en/build-with-claude/adaptive-thinking), on by default                                                               |
| Claude Opus 4.8                                          | Not supported (400 error)                                              | [Adaptive thinking](/docs/en/build-with-claude/adaptive-thinking) with [effort](/docs/en/build-with-claude/effort)                             |
| Claude Opus 4.7                                          | Not supported (400 error)                                              | [Adaptive thinking](/docs/en/build-with-claude/adaptive-thinking) with [effort](/docs/en/build-with-claude/effort)                             |
| Claude Sonnet 5                                          | Not supported (400 error)                                              | [Adaptive thinking](/docs/en/build-with-claude/adaptive-thinking) with [effort](/docs/en/build-with-claude/effort)                             |
| Claude Opus 4.6                                          | [Deprecated](/docs/en/build-with-claude/overview#feature-availability) | [Adaptive thinking](/docs/en/build-with-claude/adaptive-thinking) with [effort](/docs/en/build-with-claude/effort)                             |
| Claude Sonnet 4.6                                        | [Deprecated](/docs/en/build-with-claude/overview#feature-availability) | [Adaptive thinking](/docs/en/build-with-claude/adaptive-thinking) with [effort](/docs/en/build-with-claude/effort)                             |
| Claude Opus 4.5                                          | Supported                                                              | N/A                                                                                                                                            |
| Claude Haiku 4.5                                         | Supported                                                              | N/A                                                                                                                                            |
| Earlier Claude 4 models                                  | Supported                                                              | N/A                                                                                                                                            |

With adaptive thinking, the model decides when and how much to think on each request. On Claude Mythos Preview, Claude Fable 5, and Claude Mythos 5, `thinking: {type: "disabled"}` is not supported. For per-model behavior differences (thinking output, interleaved thinking, and block preservation), see [Differences in thinking across model versions](#differences-in-thinking-across-model-versions).

## How extended thinking works

When extended thinking is turned on, Claude creates `thinking` content blocks where it outputs its internal reasoning. Claude incorporates insights from this reasoning before crafting a final response.

The API response includes `thinking` content blocks, followed by `text` content blocks.

Here's an example of the default response format:

```json
{
  "content": [
    {
      "type": "thinking",
      "thinking": "Let me analyze this step by step...",
      "signature": "WaUjzkypQ2mUEVM36O2TxuC06KN8xyfbJwyem2dw3URve/op91XWHOEBLLqIOMfFG/UvLEczmEsUjavL...."
    },
    {
      "type": "text",
      "text": "Based on my analysis..."
    }
  ]
}
```

For more information about the response format of extended thinking, see the [Messages API Reference](/docs/en/api/messages/create).

## How to use extended thinking

Here is an example of using extended thinking in the Messages API:

<CodeGroup>
  ```bash cURL
  curl https://api.anthropic.com/v1/messages \
       --header "x-api-key: $ANTHROPIC_API_KEY" \
       --header "anthropic-version: 2023-06-01" \
       --header "content-type: application/json" \
       --data \
  '{
    "model": "claude-sonnet-4-6",
    "max_tokens": 16000,
    "thinking": {
      "type": "enabled",
      "budget_tokens": 10000
    },
    "messages": [
      {
        "role": "user",
        "content": "Are there an infinite number of prime numbers such that n mod 4 == 3?"
      }
    ]
  }'
  ```

  ```bash CLI
  ant messages create \
    --transform content --format yaml <<'YAML'
  model: claude-sonnet-4-6
  max_tokens: 16000
  thinking:
    type: enabled
    budget_tokens: 10000
  messages:
    - role: user
      content: Are there an infinite number of prime numbers such that n mod 4 == 3?
  YAML
  ```

  ```python Python
  client = anthropic.Anthropic()

  response = client.messages.create(
      model="claude-sonnet-4-6",
      max_tokens=16000,
      thinking={"type": "enabled", "budget_tokens": 10000},
      messages=[
          {
              "role": "user",
              "content": "Are there an infinite number of prime numbers such that n mod 4 == 3?",
          }
      ],
  )

  # The response contains summarized thinking blocks and text blocks
  for block in response.content:
      match block.type:
          case "thinking":
              print(f"\nThinking summary: {block.thinking}")
          case "text":
              print(f"\nResponse: {block.text}")
  ```

  ```typescript TypeScript
  const client = new Anthropic();

  const response = await client.messages.create({
    model: "claude-sonnet-4-6",
    max_tokens: 16000,
    thinking: {
      type: "enabled",
      budget_tokens: 10000,
    },
    messages: [
      {
        role: "user",
        content: "Are there an infinite number of prime numbers such that n mod 4 == 3?",
      },
    ],
  });

  // The response contains summarized thinking blocks and text blocks
  for (const block of response.content) {
    if (block.type === "thinking") {
      console.log(`\nThinking summary: ${block.thinking}`);
    } else if (block.type === "text") {
      console.log(`\nResponse: ${block.text}`);
    }
  }
  ```

  ```csharp C#
  AnthropicClient client = new();

  var response = await client.Messages.Create(new()
  {
      Model = Model.ClaudeSonnet4_6,
      MaxTokens = 16000,
      Thinking = new ThinkingConfigEnabled(budgetTokens: 10000),
      Messages =
      [
          new()
          {
              Role = Role.User,
              Content = "Are there an infinite number of prime numbers such that n mod 4 == 3?",
          },
      ],
  });

  // The response contains summarized thinking blocks and text blocks
  foreach (var block in response.Content)
  {
      if (block.TryPickThinking(out var thinking))
      {
          Console.WriteLine($"\nThinking summary: {thinking.Thinking}");
      }
      else if (block.TryPickText(out var text))
      {
          Console.WriteLine($"\nResponse: {text.Text}");
      }
  }
  ```

  ```go Go
  client := anthropic.NewClient()

  response, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
  	Model:     anthropic.ModelClaudeSonnet4_6,
  	MaxTokens: 16000,
  	Thinking:  anthropic.ThinkingConfigParamOfEnabled(10000),
  	Messages: []anthropic.MessageParam{
  		anthropic.NewUserMessage(anthropic.NewTextBlock("Are there an infinite number of prime numbers such that n mod 4 == 3?")),
  	},
  })
  if err != nil {
  	log.Fatal(err)
  }

  // The response contains summarized thinking blocks and text blocks
  for _, block := range response.Content {
  	switch block := block.AsAny().(type) {
  	case anthropic.ThinkingBlock:
  		fmt.Printf("\nThinking summary: %s", block.Thinking)
  	case anthropic.TextBlock:
  		fmt.Printf("\nResponse: %s", block.Text)
  	}
  }
  ```

  ```java Java
  import com.anthropic.client.okhttp.AnthropicOkHttpClient;
  import com.anthropic.models.messages.MessageCreateParams;
  import com.anthropic.models.messages.Model;

  void main() {
      var client = AnthropicOkHttpClient.fromEnv();

      var params = MessageCreateParams.builder()
          .model(Model.CLAUDE_SONNET_4_6)
          .maxTokens(16_000)
          .enabledThinking(10_000)
          .addUserMessage("Are there an infinite number of prime numbers such that n mod 4 == 3?")
          .build();

      var response = client.messages().create(params);

      // The response contains summarized thinking blocks and text blocks
      for (var block : response.content()) {
          block.thinking().ifPresent(thinkingBlock ->
              IO.println("\nThinking summary: " + thinkingBlock.thinking())
          );
          block.text().ifPresent(textBlock ->
              IO.println("\nResponse: " + textBlock.text())
          );
      }
  }
  ```

  ```php PHP
  $client = new Client();

  $response = $client->messages->create(
      model: 'claude-sonnet-4-6',
      maxTokens: 16000,
      thinking: ['type' => 'enabled', 'budget_tokens' => 10000],
      messages: [
          [
              'role' => 'user',
              'content' => 'Are there an infinite number of prime numbers such that n mod 4 == 3?',
          ],
      ],
  );

  // The response contains summarized thinking blocks and text blocks
  foreach ($response->content as $block) {
      echo match ($block->type) {
          'thinking' => "\nThinking summary: {$block->thinking}",
          'text' => "\nResponse: {$block->text}",
          default => '',
      };
  }
  ```

  ```ruby Ruby
  client = Anthropic::Client.new

  response = client.messages.create(
    model: "claude-sonnet-4-6",
    max_tokens: 16_000,
    thinking: {
      type: :enabled,
      budget_tokens: 10_000
    },
    messages: [
      {
        role: :user,
        content: "Are there an infinite number of prime numbers such that n mod 4 == 3?"
      }
    ]
  )

  # The response contains summarized thinking blocks and text blocks
  response.content.each do |block|
    case block
    in {type: :thinking, thinking:}
      puts "\nThinking summary: #{thinking}"
    in {type: :text, text:}
      puts "\nResponse: #{text}"
    else
    end
  end
  ```
</CodeGroup>

To turn on extended thinking, add a `thinking` object with `type` set to `enabled` and a `budget_tokens` value. On models where manual extended thinking is deprecated or not supported (see [Supported models](#supported-models)), use `type: "adaptive"` instead as described in [Adaptive thinking](/docs/en/build-with-claude/adaptive-thinking).

The `budget_tokens` parameter sets the maximum number of tokens Claude can use for its internal reasoning process. This limit applies to full thinking tokens, not to [the summarized output](#summarized-thinking). Larger budgets can improve response quality by enabling more thorough analysis for complex problems, although Claude may not use the entire budget allocated, especially at ranges above 32k.

<Warning>
  `budget_tokens` is [deprecated](/docs/en/build-with-claude/overview#feature-availability) on Claude Opus 4.6 and Claude Sonnet 4.6 and will be removed in a future model release. Use [adaptive thinking](/docs/en/build-with-claude/adaptive-thinking) with the [effort parameter](/docs/en/build-with-claude/effort) to control thinking depth instead.
</Warning>

<Note>
  [Claude Mythos Preview](https://anthropic.com/glasswing), Claude Opus 4.8, Claude Opus 4.7, Claude Sonnet 5, Claude Opus 4.6, and Claude Sonnet 4.6 support up to 128k output tokens. Claude Haiku 4.5 supports up to 64k. See the [models overview](/docs/en/about-claude/models/overview) for limits on legacy models. On the [Message Batches API](/docs/en/build-with-claude/batch-processing#extended-output-beta), the `output-300k-2026-03-24` [beta header](/docs/en/api/beta-headers) raises the output limit to 300k for Claude Opus 4.8, Opus 4.7, Sonnet 5, Opus 4.6, and Sonnet 4.6.
</Note>

`budget_tokens` must be set to a value less than `max_tokens`. However, when using [interleaved thinking with tools](#interleaved-thinking), you can exceed this limit as the token limit becomes your entire context window. Because `budget_tokens` must be less than `max_tokens`, extended thinking cannot be combined with `max_tokens: 0` ([cache pre-warming](/docs/en/build-with-claude/prompt-caching#pre-warming-the-cache)).

### Controlling thinking display

The `display` field on the thinking configuration controls how thinking content is returned in API responses. It accepts two values:

* `"summarized"`: Thinking blocks contain summarized thinking text. See [Summarized thinking](#summarized-thinking) for details. This is the default on Claude Opus 4.6, Claude Sonnet 4.6, and earlier Claude 4 models.
* `"omitted"`: Thinking blocks are returned with an empty `thinking` field. The `signature` field still carries the encrypted full thinking for multi-turn continuity (see [Thinking encryption](#thinking-encryption)). This is the default on Claude Fable 5, Claude Mythos 5, Claude Sonnet 5, Claude Opus 4.8, Claude Opus 4.7, and [Claude Mythos Preview](https://anthropic.com/glasswing).

Setting `display: "omitted"` is useful when your application doesn't surface thinking content to users. The primary benefit is **faster time-to-first-text-token when streaming:** The server skips streaming thinking tokens entirely and delivers only the signature, so the final text response begins streaming sooner.

Here are some important considerations for omitted thinking:

* You're still charged for the full thinking tokens. Omitting reduces latency, not cost.
* If you pass thinking blocks back in multi-turn conversations, pass them unchanged. The server decrypts the `signature` to reconstruct the original thinking for prompt construction (see [Preserving thinking blocks](/docs/en/build-with-claude/extended-thinking#preserving-thinking-blocks)). Any text you place in the `thinking` field of a round-tripped omitted block is ignored.
* `display` is invalid with `thinking.type: "disabled"` (there is nothing to display).
* When using `thinking.type: "adaptive"` and the model skips thinking for a simple request, no thinking block is produced regardless of `display`.

<Note>
  The `signature` field is identical whether `display` is `"summarized"` or `"omitted"`. Switching `display` values between turns in a conversation is supported.
</Note>

<Note>
  On [Claude Mythos Preview](https://anthropic.com/glasswing), `display` defaults to `"omitted"`. The examples in this section pass `display` explicitly so they apply to all models, but on Mythos Preview you can leave it unset and receive the same behavior. To receive summarized thinking on Mythos Preview, set `display: "summarized"` explicitly.
</Note>

Automated pipelines that never surface thinking content to end users can skip the overhead of receiving thinking tokens over the wire. Latency-sensitive applications get the same reasoning quality without waiting for thinking text to stream before the final response begins.

<CodeGroup>
  ```bash cURL
  curl https://api.anthropic.com/v1/messages \
       --header "x-api-key: $ANTHROPIC_API_KEY" \
       --header "anthropic-version: 2023-06-01" \
       --header "content-type: application/json" \
       --data \
  '{
      "model": "claude-sonnet-4-6",
      "max_tokens": 16000,
      "thinking": {
          "type": "enabled",
          "budget_tokens": 10000,
          "display": "omitted"
      },
      "messages": [
          {
              "role": "user",
              "content": "What is 27 * 453?"
          }
      ]
  }'
  ```

  ```bash CLI
  ant messages create \
    --model claude-sonnet-4-6 \
    --max-tokens 16000 \
    --transform content \
    --thinking '{type: enabled, budget_tokens: 10000, display: omitted}' \
    --message '{role: user, content: "What is 27 * 453?"}' \
    --format yaml
  ```

  ```python Python
  client = anthropic.Anthropic()

  response = client.messages.create(
      model="claude-sonnet-4-6",
      max_tokens=16000,
      thinking={
          "type": "enabled",
          "budget_tokens": 10000,
          "display": "omitted",
      },
      messages=[
          {"role": "user", "content": "What is 27 * 453?"},
      ],
  )

  for block in response.content:
      if block.type == "thinking":
          if block.thinking:
              print(f"Thinking: {block.thinking}")
          else:
              print("Thinking: [omitted]")
      elif block.type == "text":
          print(f"Response: {block.text}")
  ```

  ```typescript TypeScript
  const client = new Anthropic();

  const response = await client.messages.create({
    model: "claude-sonnet-4-6",
    max_tokens: 16000,
    thinking: {
      type: "enabled",
      budget_tokens: 10000,
      display: "omitted"
    },
    messages: [
      {
        role: "user",
        content: "What is 27 * 453?"
      }
    ]
  });

  for (const block of response.content) {
    if (block.type === "thinking") {
      if (block.thinking.length > 0) {
        console.log(`Thinking: ${block.thinking}`);
      } else {
        console.log("Thinking: [omitted]");
      }
    } else if (block.type === "text") {
      console.log(`Response: ${block.text}`);
    }
  }
  ```

  ```csharp C#
  AnthropicClient client = new();

  var message = await client.Messages.Create(new MessageCreateParams
  {
      Model = Model.ClaudeSonnet4_6,
      MaxTokens = 16000,
      Thinking = new ThinkingConfigEnabled
      {
          BudgetTokens = 10000,
          Display = ThinkingConfigEnabledDisplay.Omitted
      },
      Messages =
      [
          new() { Role = Role.User, Content = "What is 27 * 453?" }
      ]
  });

  foreach (var block in message.Content)
  {
      if (block.TryPickThinking(out ThinkingBlock? thinking))
      {
          Console.WriteLine(string.IsNullOrEmpty(thinking.Thinking)
              ? "Thinking: [omitted]"
              : $"Thinking: {thinking.Thinking}");
      }
      else if (block.TryPickText(out TextBlock? text))
      {
          Console.WriteLine($"Response: {text.Text}");
      }
  }
  ```

  ```go Go
  client := anthropic.NewClient()

  response, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
  	Model:     anthropic.ModelClaudeSonnet4_6,
  	MaxTokens: 16000,
  	Thinking: anthropic.ThinkingConfigParamUnion{
  		OfEnabled: &anthropic.ThinkingConfigEnabledParam{
  			BudgetTokens: 10000,
  			Display:      anthropic.ThinkingConfigEnabledDisplayOmitted,
  		},
  	},
  	Messages: []anthropic.MessageParam{
  		anthropic.NewUserMessage(anthropic.NewTextBlock("What is 27 * 453?")),
  	},
  })
  if err != nil {
  	log.Fatal(err)
  }

  for _, block := range response.Content {
  	switch v := block.AsAny().(type) {
  	case anthropic.ThinkingBlock:
  		fmt.Println("Thinking:", cmp.Or(v.Thinking, "[omitted]"))
  	case anthropic.TextBlock:
  		fmt.Println("Response:", v.Text)
  	}
  }
  ```

  ```java Java
  import com.anthropic.models.messages.ThinkingConfigEnabled;
  // ...
  void main() {
      AnthropicClient client = AnthropicOkHttpClient.fromEnv();

      MessageCreateParams params = MessageCreateParams.builder()
          .model(Model.CLAUDE_SONNET_4_6)
          .maxTokens(16000L)
          .thinking(ThinkingConfigEnabled.builder()
              .budgetTokens(10000L)
              .display(ThinkingConfigEnabled.Display.OMITTED)
              .build())
          .addUserMessage("What is 27 * 453?")
          .build();

      Message message = client.messages().create(params);

      message.content().forEach(block -> {
          block.thinking().ifPresent(thinkingBlock -> {
              if (thinkingBlock.thinking().isEmpty()) {
                  IO.println("Thinking: [omitted]");
              } else {
                  IO.println("Thinking: " + thinkingBlock.thinking());
              }
          });
          block.text().ifPresent(textBlock ->
              IO.println("Response: " + textBlock.text())
          );
      });
  }
  ```

  ```php PHP
  use Anthropic\Messages\TextBlock;
  use Anthropic\Messages\ThinkingBlock;
  use Anthropic\Messages\ThinkingConfigEnabled;
  use Anthropic\Messages\ThinkingConfigEnabled\Display;
  // ...
  $client = new Client();

  $response = $client->messages->create(
      model: 'claude-sonnet-4-6',
      maxTokens: 16000,
      thinking: ThinkingConfigEnabled::with(
          budgetTokens: 10000,
          display: Display::OMITTED,
      ),
      messages: [
          ['role' => 'user', 'content' => 'What is 27 * 453?'],
      ],
  );

  foreach ($response->content as $block) {
      echo match (true) {
          $block instanceof ThinkingBlock && $block->thinking === '' => "Thinking: [omitted]\n",
          $block instanceof ThinkingBlock => "Thinking: {$block->thinking}\n",
          $block instanceof TextBlock => "Response: {$block->text}\n",
          default => '',
      };
  }
  ```

  ```ruby Ruby
  client = Anthropic::Client.new

  response = client.messages.create(
    model: "claude-sonnet-4-6",
    max_tokens: 16000,
    thinking: {
      type: :enabled,
      budget_tokens: 10000,
      # The Ruby SDK uses `display_` (trailing underscore) to avoid
      # shadowing Kernel#display; the wire field is still `display`.
      display_: :omitted
    },
    messages: [{role: "user", content: "What is 27 * 453?"}]
  )

  response.content.each do |block|
    case block.type
    when :thinking
      puts block.thinking.empty? ? "Thinking: [omitted]" : "Thinking: #{block.thinking}"
    when :text
      puts "Response: #{block.text}"
    end
  end
  ```
</CodeGroup>

When `display: "omitted"` is set, the response contains `thinking` blocks with an empty `thinking` field:

```json Output
{
  "content": [
    {
      "type": "thinking",
      "thinking": "",
      "signature": "EosnCkYICxIMMb3LzNrMu..."
    },
    {
      "type": "text",
      "text": "The answer is 12,231."
    }
  ]
}
```

When streaming with `display: "omitted"`, no `thinking_delta` events are emitted; see [Streaming thinking](#streaming-thinking) for the event sequence.

### Summarized thinking

With extended thinking enabled, the Messages API for Claude 4 models returns a summary of Claude's full thinking process. Summarized thinking provides the full intelligence benefits of extended thinking, while preventing misuse. This is the default behavior on Claude 4 models when the `display` field on the thinking configuration is unset or set to `"summarized"`. On Claude Fable 5, Claude Mythos 5, Claude Sonnet 5, Claude Opus 4.8, Claude Opus 4.7, and [Claude Mythos Preview](https://anthropic.com/glasswing), `display` defaults to `"omitted"` instead, so you must set `display: "summarized"` explicitly to receive summarized thinking.

Here are some important considerations for summarized thinking:

* You're charged for the full thinking tokens generated by the original request, not the summary tokens.
* The billed output token count will **not match** the count of tokens you see in the response.
* On Claude 4 models, the first few lines of thinking output are more verbose, providing detailed reasoning that's particularly helpful for prompt engineering purposes. [Claude Mythos Preview](https://anthropic.com/glasswing) summarizes from the first token, so its thinking blocks do not show this verbose preamble.
* As Anthropic seeks to improve the extended thinking feature, summarization behavior is subject to change.
* Summarization preserves the key ideas of Claude's thinking process with minimal added latency, enabling a streamable user experience.
* Summarization is processed by a different model than the one you target in your requests. The thinking model does not see the summarized output.

<Note>
  In rare cases where you need access to full thinking output for Claude 4 models, [contact Anthropic sales](mailto:sales@anthropic.com).
</Note>

### Streaming thinking

You can stream extended thinking responses using [server-sent events (SSE)](https://developer.mozilla.org/en-US/Web/API/Server-sent%5Fevents/Using%5Fserver-sent%5Fevents).

When streaming is enabled for extended thinking, you receive thinking content via `thinking_delta` events.

When `display: "omitted"` is set, no `thinking_delta` events are emitted. See [Controlling thinking display](#controlling-thinking-display).

For more documentation on streaming via the Messages API, see [Streaming Messages](/docs/en/build-with-claude/streaming).

Here's how to handle streaming with thinking:

<CodeGroup>
  ```bash cURL
  curl https://api.anthropic.com/v1/messages \
       --header "x-api-key: $ANTHROPIC_API_KEY" \
       --header "anthropic-version: 2023-06-01" \
       --header "content-type: application/json" \
       --data \
  '{
      "model": "claude-sonnet-4-6",
      "max_tokens": 16000,
      "stream": true,
      "thinking": {
          "type": "enabled",
          "budget_tokens": 10000
      },
      "messages": [
          {
              "role": "user",
              "content": "What is the greatest common divisor of 1071 and 462?"
          }
      ]
  }'
  ```

  ```bash CLI
  ant messages create \
    --stream \
    --model claude-sonnet-4-6 \
    --max-tokens 16000 \
    --thinking '{type: enabled, budget_tokens: 10000}' \
    --message '{role: user, content: What is the greatest common divisor of 1071 and 462?}' \
    --format jsonl
  ```

  ```python Python
  client = anthropic.Anthropic()

  with client.messages.stream(
      model="claude-sonnet-4-6",
      max_tokens=16000,
      thinking={"type": "enabled", "budget_tokens": 10000},
      messages=[
          {
              "role": "user",
              "content": "What is the greatest common divisor of 1071 and 462?",
          }
      ],
  ) as stream:
      thinking_started = False
      response_started = False

      for event in stream:
          if event.type == "content_block_start":
              print(f"\nStarting {event.content_block.type} block...")
              # Reset flags for each new block
              thinking_started = False
              response_started = False
          elif event.type == "content_block_delta":
              if event.delta.type == "thinking_delta":
                  if not thinking_started:
                      print("Thinking: ", end="", flush=True)
                      thinking_started = True
                  print(event.delta.thinking, end="", flush=True)
              elif event.delta.type == "text_delta":
                  if not response_started:
                      print("Response: ", end="", flush=True)
                      response_started = True
                  print(event.delta.text, end="", flush=True)
          elif event.type == "content_block_stop":
              print("\nBlock complete.")
  ```

  ```typescript TypeScript
  const client = new Anthropic();

  const stream = await client.messages.stream({
    model: "claude-sonnet-4-6",
    max_tokens: 16000,
    thinking: {
      type: "enabled",
      budget_tokens: 10000
    },
    messages: [
      {
        role: "user",
        content: "What is the greatest common divisor of 1071 and 462?"
      }
    ]
  });

  let thinkingStarted = false;
  let responseStarted = false;

  for await (const event of stream) {
    if (event.type === "content_block_start") {
      console.log(`\nStarting ${event.content_block.type} block...`);
      // Reset flags for each new block
      thinkingStarted = false;
      responseStarted = false;
    } else if (event.type === "content_block_delta") {
      if (event.delta.type === "thinking_delta") {
        if (!thinkingStarted) {
          process.stdout.write("Thinking: ");
          thinkingStarted = true;
        }
        process.stdout.write(event.delta.thinking);
      } else if (event.delta.type === "text_delta") {
        if (!responseStarted) {
          process.stdout.write("Response: ");
          responseStarted = true;
        }
        process.stdout.write(event.delta.text);
      }
    } else if (event.type === "content_block_stop") {
      console.log("\nBlock complete.");
    }
  }
  ```

  ```csharp C#
  AnthropicClient client = new();

  var parameters = new MessageCreateParams
  {
      Model = Model.ClaudeSonnet4_6,
      MaxTokens = 16000,
      Thinking = new ThinkingConfigEnabled(budgetTokens: 10000),
      Messages = [new() { Role = Role.User, Content = "What is the greatest common divisor of 1071 and 462?" }]
  };

  bool thinkingStarted = false;
  bool responseStarted = false;

  await foreach (var streamEvent in client.Messages.CreateStreaming(parameters))
  {
      if (streamEvent.TryPickContentBlockStart(out var blockStart))
      {
          Console.WriteLine($"\nStarting {blockStart.ContentBlock.Type} block...");
          thinkingStarted = false;
          responseStarted = false;
      }
      else if (streamEvent.TryPickContentBlockDelta(out var blockDelta))
      {
          if (blockDelta.Delta.TryPickThinking(out var thinkingDelta))
          {
              if (!thinkingStarted)
              {
                  Console.Write("Thinking: ");
                  thinkingStarted = true;
              }
              Console.Write(thinkingDelta.Thinking);
          }
          else if (blockDelta.Delta.TryPickText(out var textDelta))
          {
              if (!responseStarted)
              {
                  Console.Write("Response: ");
                  responseStarted = true;
              }
              Console.Write(textDelta.Text);
          }
      }
      else if (streamEvent.TryPickContentBlockStop(out _))
      {
          Console.WriteLine("\nBlock complete.");
      }
  }
  ```

  ```go Go
  client := anthropic.NewClient()

  stream := client.Messages.NewStreaming(context.TODO(), anthropic.MessageNewParams{
  	Model:     anthropic.ModelClaudeSonnet4_6,
  	MaxTokens: 16000,
  	Thinking:  anthropic.ThinkingConfigParamOfEnabled(10000),
  	Messages: []anthropic.MessageParam{
  		anthropic.NewUserMessage(anthropic.NewTextBlock("What is the greatest common divisor of 1071 and 462?")),
  	},
  })

  thinkingStarted := false
  responseStarted := false

  for stream.Next() {
  	event := stream.Current()
  	switch eventVariant := event.AsAny().(type) {
  	case anthropic.ContentBlockStartEvent:
  		fmt.Printf("\nStarting %s block...\n", eventVariant.ContentBlock.Type)
  		thinkingStarted = false
  		responseStarted = false
  	case anthropic.ContentBlockDeltaEvent:
  		switch deltaVariant := eventVariant.Delta.AsAny().(type) {
  		case anthropic.ThinkingDelta:
  			if !thinkingStarted {
  				fmt.Print("Thinking: ")
  				thinkingStarted = true
  			}
  			fmt.Print(deltaVariant.Thinking)
  		case anthropic.TextDelta:
  			if !responseStarted {
  				fmt.Print("Response: ")
  				responseStarted = true
  			}
  			fmt.Print(deltaVariant.Text)
  		}
  	case anthropic.ContentBlockStopEvent:
  		fmt.Println("\nBlock complete.")
  	}
  }

  if err := stream.Err(); err != nil {
  	log.Fatal(err)
  }
  ```

  ```java Java
  AnthropicClient client = AnthropicOkHttpClient.fromEnv();

  MessageCreateParams params = MessageCreateParams.builder()
      .model(Model.CLAUDE_SONNET_4_6)
      .maxTokens(16000L)
      .enabledThinking(10000L)
      .addUserMessage("What is the greatest common divisor of 1071 and 462?")
      .build();

  try (var streamResponse = client.messages().createStreaming(params)) {
      streamResponse.stream().forEach(event -> {
          event.contentBlockStart().ifPresent(startEvent ->
              IO.println("\nStarting block...")
          );
          event.contentBlockDelta().ifPresent(deltaEvent -> {
              deltaEvent.delta().thinking().ifPresent(td ->
                  IO.print(td.thinking())
              );
              deltaEvent.delta().text().ifPresent(td ->
                  IO.print(td.text())
              );
          });
          event.contentBlockStop().ifPresent(stopEvent ->
              IO.println("\nBlock complete.")
          );
      });
  }
  ```

  ```php PHP
  $client = new Client();

  $thinkingStarted = false;
  $responseStarted = false;

  $stream = $client->messages->createStream(
      maxTokens: 16000,
      messages: [
          ['role' => 'user', 'content' => 'What is the greatest common divisor of 1071 and 462?']
      ],
      model: 'claude-sonnet-4-6',
      thinking: ['type' => 'enabled', 'budget_tokens' => 10000],
  );

  foreach ($stream as $event) {
      if ($event->type === 'content_block_start') {
          echo "\nStarting {$event->contentBlock->type} block...\n";
          $thinkingStarted = false;
          $responseStarted = false;
      } elseif ($event->type === 'content_block_delta') {
          if ($event->delta->type === 'thinking_delta') {
              if (!$thinkingStarted) {
                  echo "Thinking: ";
                  $thinkingStarted = true;
              }
              echo $event->delta->thinking;
          } elseif ($event->delta->type === 'text_delta') {
              if (!$responseStarted) {
                  echo "Response: ";
                  $responseStarted = true;
              }
              echo $event->delta->text;
          }
      } elseif ($event->type === 'content_block_stop') {
          echo "\nBlock complete.\n";
      }
  }
  ```

  ```ruby Ruby
  client = Anthropic::Client.new

  thinking_started = false
  response_started = false

  stream = client.messages.stream(
    model: "claude-sonnet-4-6",
    max_tokens: 16000,
    thinking: {
      type: "enabled",
      budget_tokens: 10000
    },
    messages: [
      { role: "user", content: "What is the greatest common divisor of 1071 and 462?" }
    ]
  )

  stream.each do |event|
    case event.type
    when :content_block_start
      puts "\nStarting #{event.content_block.type} block..."
      thinking_started = false
      response_started = false
    when :content_block_delta
      if event.delta.type == :thinking_delta
        unless thinking_started
          print "Thinking: "
          thinking_started = true
        end
        print event.delta.thinking
      elsif event.delta.type == :text_delta
        unless response_started
          print "Response: "
          response_started = true
        end
        print event.delta.text
      end
    when :content_block_stop
      puts "\nBlock complete."
    end
  end
  ```
</CodeGroup>

Example streaming output:

```sse Output
event: message_start
data: {"type": "message_start", "message": {"id": "msg_01...", "type": "message", "role": "assistant", "content": [], "model": "claude-sonnet-4-6", "stop_reason": null, "stop_sequence": null}}

event: content_block_start
data: {"type": "content_block_start", "index": 0, "content_block": {"type": "thinking", "thinking": "", "signature": ""}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "I need to find the GCD of 1071 and 462 using the Euclidean algorithm.\n\n1071 = 2 × 462 + 147"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "\n462 = 3 × 147 + 21\n147 = 7 × 21 + 0\n\nSo GCD(1071, 462) = 21"}}

// Additional thinking deltas...

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "signature_delta", "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxBdjrkzLoky3dl1pkiMOYds..."}}

event: content_block_stop
data: {"type": "content_block_stop", "index": 0}

event: content_block_start
data: {"type": "content_block_start", "index": 1, "content_block": {"type": "text", "text": ""}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 1, "delta": {"type": "text_delta", "text": "The greatest common divisor of 1071 and 462 is **21**."}}

// Additional text deltas...

event: content_block_stop
data: {"type": "content_block_stop", "index": 1}

event: message_delta
data: {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": null}}

event: message_stop
data: {"type": "message_stop"}
```

When `display: "omitted"` is set, the thinking block opens, a single `signature_delta` arrives, and the block closes without any `thinking_delta` events. Text streaming begins immediately after:

```sse Output
event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"EosnCkYICxIMMb3LzNrMu..."}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}
```

<Note>
  When using streaming with thinking enabled, you might notice that text sometimes arrives in larger chunks alternating with smaller, token-by-token delivery. This is expected behavior, especially for thinking content.

  The streaming system needs to process content in batches for optimal performance, which can result in this "chunky" delivery pattern, with possible delays between streaming events.
</Note>

## Extended thinking with tool use

Extended thinking can be used alongside [tool use](/docs/en/agents-and-tools/tool-use/overview), allowing Claude to reason through tool selection and results processing.

When using extended thinking with tool use, be aware of the following limitations:

1. **Tool choice limitation**: Tool use with thinking only supports `tool_choice: {"type": "auto"}` (the default) or `tool_choice: {"type": "none"}`. Using `tool_choice: {"type": "any"}` or `tool_choice: {"type": "tool", "name": "..."}` will result in an error because these options force tool use, which is incompatible with extended thinking.

2. **Preserving thinking blocks**: During tool use, you must pass `thinking` blocks back to the API for the last assistant message. Include the complete unmodified block back to the API to maintain reasoning continuity.

### Toggling thinking modes in conversations

You can't toggle thinking in the middle of an assistant turn, including during tool use loops. The entire assistant turn should operate in a single thinking mode:

* **If thinking is enabled**, the final assistant turn should start with a thinking block.
* **If thinking is disabled**, the final assistant turn shouldn't contain any thinking blocks

From the model's perspective, **tool use loops are part of the assistant turn**. An assistant turn doesn't complete until Claude finishes its full response, which may include multiple tool calls and results.

For example, this sequence is all part of a **single assistant turn**:

```text wrap
User: "What's the weather in Paris?"
Assistant: [thinking] + [tool_use: get_weather]
User: [tool_result: "20°C, sunny"]
Assistant: [text: "The weather in Paris is 20°C and sunny"]
```

Even though there are multiple API messages, the tool use loop is conceptually part of one continuous assistant response.

#### Graceful thinking degradation

When a mid-turn thinking conflict occurs (such as toggling thinking on or off during a tool use loop), the API automatically disables thinking for that request. To preserve model quality and remain on-distribution, the API may:

* Strip thinking blocks from the conversation when they would create an invalid turn structure
* Disable thinking for the current request when the conversation history is incompatible with thinking being enabled

This means that attempting to toggle thinking mid-turn won't cause an error, but thinking will be silently disabled for that request. To confirm whether thinking was active, check for the presence of `thinking` blocks in the response.

#### Practical guidance

**Best practice**: Plan your thinking strategy at the start of each turn rather than trying to toggle mid-turn.

**Example: Toggling thinking after completing a turn**

```text wrap
User: "What's the weather?"
Assistant: [tool_use] (thinking disabled)
User: [tool_result]
Assistant: [text: "It's sunny"]
User: "What about tomorrow?"
Assistant: [thinking] + [text: "..."] (thinking enabled - new turn)
```

By completing the assistant turn before toggling thinking, you ensure that thinking is actually enabled for the new request.

<Note>
  Toggling thinking modes also invalidates prompt caching for message history. For more details, see the [Extended thinking with prompt caching](#extended-thinking-with-prompt-caching) section.
</Note>

<AccordionGroup>
  <Accordion title="Example: Passing thinking blocks with tool results">
    Here's a practical example showing how to preserve thinking blocks when providing tool results:

    <CodeGroup>
      ```bash CLI
      ant messages create --transform content <<'YAML'
      model: claude-sonnet-4-6
      max_tokens: 16000
      thinking:
        type: enabled
        budget_tokens: 10000
      tools:
        - name: get_weather
          description: Get current weather for a location
          input_schema:
            type: object
            properties:
              location:
                type: string
                description: City name
            required:
              - location
      messages:
        - role: user
          content: "What's the weather in Paris?"
      YAML
      ```

      ```python Python

      client = anthropic.Anthropic()

      weather_tool = {
          "name": "get_weather",
          "description": "Get current weather for a location",
          "input_schema": {
              "type": "object",
              "properties": {"location": {"type": "string", "description": "City name"}},
              "required": ["location"],
          },
      }

      # First request - Claude responds with thinking and tool request
      response = client.messages.create(
          model="claude-sonnet-4-6",
          max_tokens=16000,
          thinking={"type": "enabled", "budget_tokens": 10000},
          tools=[weather_tool],
          messages=[{"role": "user", "content": "What's the weather in Paris?"}],
      )
      ```

      ```typescript TypeScript
      const client = new Anthropic();

      const weatherTool: Anthropic.Tool = {
        name: "get_weather",
        description: "Get current weather for a location",
        input_schema: {
          type: "object",
          properties: {
            location: { type: "string", description: "City name" }
          },
          required: ["location"]
        }
      };

      // First request - Claude responds with thinking and tool request
      const response = await client.messages.create({
        model: "claude-sonnet-4-6",
        max_tokens: 16000,
        thinking: {
          type: "enabled",
          budget_tokens: 10000
        },
        tools: [weatherTool],
        messages: [{ role: "user", content: "What's the weather in Paris?" }]
      });
      ```

      ```csharp C#
      AnthropicClient client = new();

      var weatherTool = new ToolUnion(new Tool()
      {
          Name = "get_weather",
          Description = "Get current weather for a location",
          InputSchema = new InputSchema()
          {
              Properties = new Dictionary<string, JsonElement>
              {
                  ["location"] = JsonSerializer.SerializeToElement(new { type = "string", description = "City name" }),
              },
              Required = ["location"],
          },
      });

      var parameters = new MessageCreateParams
      {
          Model = Model.ClaudeSonnet4_6,
          MaxTokens = 16000,
          Thinking = new ThinkingConfigEnabled(budgetTokens: 10000),
          Tools = [weatherTool],
          Messages = [new() { Role = Role.User, Content = "What's the weather in Paris?" }]
      };

      var message = await client.Messages.Create(parameters);
      Console.WriteLine(message);
      ```

      ```go Go
      client := anthropic.NewClient()

      weatherTool := anthropic.ToolUnionParam{
      	OfTool: &anthropic.ToolParam{
      		Name:        "get_weather",
      		Description: anthropic.String("Get current weather for a location"),
      		InputSchema: anthropic.ToolInputSchemaParam{
      			Properties: map[string]any{
      				"location": map[string]any{
      					"type":        "string",
      					"description": "City name",
      				},
      			},
      			Required: []string{"location"},
      		},
      	},
      }

      response, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
      	Model:     anthropic.ModelClaudeSonnet4_6,
      	MaxTokens: 16000,
      	Thinking:  anthropic.ThinkingConfigParamOfEnabled(10000),
      	Tools:     []anthropic.ToolUnionParam{weatherTool},
      	Messages: []anthropic.MessageParam{
      		anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather in Paris?")),
      	},
      })
      if err != nil {
      	log.Fatal(err)
      }
      fmt.Println(response)
      ```

      ```java Java
      AnthropicClient client = AnthropicOkHttpClient.fromEnv();

      MessageCreateParams params = MessageCreateParams.builder()
          .model(Model.CLAUDE_SONNET_4_6)
          .maxTokens(16000L)
          .enabledThinking(10000L)
          .addTool(Tool.builder()
              .name("get_weather")
              .description("Get current weather for a location")
              .inputSchema(Tool.InputSchema.builder()
                  .properties(JsonValue.from(Map.of(
                      "location", Map.of("type", "string", "description", "City name")
                  )))
                  .required(List.of("location"))
                  .build())
              .build())
          .addUserMessage("What's the weather in Paris?")
          .build();

      Message response = client.messages().create(params);
      IO.println(response);
      ```

      ```php PHP
      $client = new Client();

      $weatherTool = [
          'name' => 'get_weather',
          'description' => 'Get current weather for a location',
          'input_schema' => [
              'type' => 'object',
              'properties' => [
                  'location' => ['type' => 'string', 'description' => 'City name']
              ],
              'required' => ['location']
          ]
      ];

      $message = $client->messages->create(
          maxTokens: 16000,
          messages: [
              ['role' => 'user', 'content' => "What's the weather in Paris?"]
          ],
          model: 'claude-sonnet-4-6',
          thinking: ['type' => 'enabled', 'budget_tokens' => 10000],
          tools: [$weatherTool],
      );
      echo $message;
      ```

      ```ruby Ruby
      client = Anthropic::Client.new

      weather_tool = {
        name: "get_weather",
        description: "Get current weather for a location",
        input_schema: {
          type: "object",
          properties: {
            location: { type: "string", description: "City name" }
          },
          required: ["location"]
        }
      }

      message = client.messages.create(
        model: "claude-sonnet-4-6",
        max_tokens: 16000,
        thinking: {
          type: "enabled",
          budget_tokens: 10000
        },
        tools: [weather_tool],
        messages: [
          { role: "user", content: "What's the weather in Paris?" }
        ]
      )
      puts message
      ```
    </CodeGroup>

    The API response includes thinking, text, and tool\_use blocks:

    ```json Output
    {
      "content": [
        {
          "type": "thinking",
          "thinking": "The user wants to know the current weather in Paris. I have access to a function `get_weather`...",
          "signature": "BDaL4VrbR2Oj0hO4XpJxT28J5TILnCrrUXoKiiNBZW9P+nr8XSj1zuZzAl4egiCCpQNvfyUuFFJP5CncdYZEQPPmLxYsNrcs...."
        },
        {
          "type": "text",
          "text": "I can help you get the current weather information for Paris. Let me check that for you"
        },
        {
          "type": "tool_use",
          "id": "toolu_01CswdEQBMshySk6Y9DFKrfq",
          "name": "get_weather",
          "input": {
            "location": "Paris"
          }
        }
      ]
    }
    ```

    Now let's continue the conversation and use the tool

    <CodeGroup>
      ```bash CLI
      # First turn: capture the assistant content array (thinking + tool_use,
      # with signatures intact) as compact JSON.
      ASSISTANT_CONTENT=$(ant messages create \
        --transform content <<'YAML'
      model: claude-sonnet-4-6
      max_tokens: 16000
      thinking:
        type: enabled
        budget_tokens: 10000
      tools:
        - name: get_weather
          description: Get current weather for a location
          input_schema:
            type: object
            properties:
              location:
                type: string
                description: City name
            required: [location]
      messages:
        - role: user
          content: What's the weather in Paris?
      YAML
      )

      TOOL_USE_ID=$(printf '%s' "$ASSISTANT_CONTENT" \
        | jq -r '.[] | select(.type == "tool_use") | .id')

      # Second turn: pass the captured blocks back as the assistant message.
      # The thinking block MUST accompany the tool_use block.
      ant messages create <<YAML
      model: claude-sonnet-4-6
      max_tokens: 16000
      thinking:
        type: enabled
        budget_tokens: 10000
      tools:
        - name: get_weather
          description: Get current weather for a location
          input_schema:
            type: object
            properties:
              location:
                type: string
                description: City name
            required: [location]
      messages:
        - role: user
          content: What's the weather in Paris?
        - role: assistant
          content: $ASSISTANT_CONTENT
        - role: user
          content:
            - type: tool_result
              tool_use_id: $TOOL_USE_ID
              content: "Current temperature: 88°F"
      YAML
      ```

      ```python Python

      client = anthropic.Anthropic()
      weather_tool = {
          "name": "get_weather",
          "description": "Get current weather for a location",
          "input_schema": {
              "type": "object",
              "properties": {"location": {"type": "string", "description": "City name"}},
              "required": ["location"],
          },
      }
      response = client.messages.create(
          model="claude-sonnet-4-6",
          max_tokens=16000,
          thinking={"type": "enabled", "budget_tokens": 10000},
          tools=[weather_tool],
          messages=[{"role": "user", "content": "What's the weather in Paris?"}],
      )
      # Extract thinking block and tool use block
      thinking_block = next(
          (block for block in response.content if block.type == "thinking"), None
      )
      tool_use_block = next(
          (block for block in response.content if block.type == "tool_use"), None
      )

      # Call your actual weather API, here is where your actual API call would go
      # Let's pretend this is what we get back
      weather_data = {"temperature": 88}

      # Second request - Include thinking block and tool result
      # No new thinking blocks are generated in the response
      continuation = client.messages.create(
          model="claude-sonnet-4-6",
          max_tokens=16000,
          thinking={"type": "enabled", "budget_tokens": 10000},
          tools=[weather_tool],
          messages=[
              {"role": "user", "content": "What's the weather in Paris?"},
              # notice that the thinking_block is passed in as well as the tool_use_block
              # if this is not passed in, an error is raised
              {"role": "assistant", "content": [thinking_block, tool_use_block]},
              {
                  "role": "user",
                  "content": [
                      {
                          "type": "tool_result",
                          "tool_use_id": tool_use_block.id,
                          "content": f"Current temperature: {weather_data['temperature']}°F",
                      }
                  ],
              },
          ],
      )
      print(continuation)
      ```

      ```typescript TypeScript
      const client = new Anthropic();

      const weatherTool: Anthropic.Tool = {
        name: "get_weather",
        description: "Get current weather for a location",
        input_schema: {
          type: "object",
          properties: {
            location: { type: "string", description: "City name" }
          },
          required: ["location"]
        }
      };

      const response = await client.messages.create({
        model: "claude-sonnet-4-6",
        max_tokens: 16000,
        thinking: {
          type: "enabled",
          budget_tokens: 10000
        },
        tools: [weatherTool],
        messages: [{ role: "user", content: "What's the weather in Paris?" }]
      });

      // Extract thinking block and tool use block
      const thinkingBlock = response.content.find(
        (block): block is Anthropic.ThinkingBlock => block.type === "thinking"
      );
      const toolUseBlock = response.content.find(
        (block): block is Anthropic.ToolUseBlock => block.type === "tool_use"
      );

      // Call your actual weather API, here is where your actual API call would go
      // Let's pretend this is what we get back
      const weatherData = { temperature: 88 };

      if (thinkingBlock && toolUseBlock) {
        // Second request - Include thinking block and tool result
        // No new thinking blocks are generated in the response
        const continuation = await client.messages.create({
          model: "claude-sonnet-4-6",
          max_tokens: 16000,
          thinking: {
            type: "enabled",
            budget_tokens: 10000
          },
          tools: [weatherTool],
          messages: [
            { role: "user", content: "What's the weather in Paris?" },
            // notice that the thinkingBlock is passed in as well as the toolUseBlock
            // if this is not passed in, an error is raised
            { role: "assistant", content: [thinkingBlock, toolUseBlock] },
            {
              role: "user",
              content: [
                {
                  type: "tool_result" as const,
                  tool_use_id: toolUseBlock.id,
                  content: `Current temperature: ${weatherData.temperature}°F`
                }
              ]
            }
          ]
        });
        console.log(continuation);
      }
      ```

      ```csharp C#
      AnthropicClient client = new();

      var weatherTool = new ToolUnion(new Tool()
      {
          Name = "get_weather",
          Description = "Get current weather for a location",
          InputSchema = new InputSchema()
          {
              Properties = new Dictionary<string, JsonElement>
              {
                  ["location"] = JsonSerializer.SerializeToElement(new { type = "string", description = "City name" }),
              },
              Required = ["location"],
          },
      });

      var parameters = new MessageCreateParams
      {
          Model = Model.ClaudeSonnet4_6,
          MaxTokens = 16000,
          Thinking = new ThinkingConfigEnabled(budgetTokens: 10000),
          Tools = [weatherTool],
          Messages = [
              new() { Role = Role.User, Content = "What's the weather in Paris?" }
          ]
      };

      var response = await client.Messages.Create(parameters);

      // Extract the tool_use block to get its ID for the tool result
      ToolUseBlock? toolUseBlock = null;
      foreach (var block in response.Content)
      {
          if (block.TryPickToolUse(out var toolUse))
          {
              toolUseBlock = toolUse;
          }
      }

      var weatherData = new { temperature = 88 };

      // Build continuation with tool result
      var continuationParams = new MessageCreateParams
      {
          Model = Model.ClaudeSonnet4_6,
          MaxTokens = 16000,
          Thinking = new ThinkingConfigEnabled(budgetTokens: 10000),
          Tools = [weatherTool],
          Messages = [
              new() { Role = Role.User, Content = "What's the weather in Paris?" },
              // response.Content includes the thinking blocks; passing them back is required
              new() { Role = Role.Assistant, Content = response.Content.Select(block => new ContentBlockParam(block.Json)).ToList() },
              new() { Role = Role.User, Content = new MessageParamContent(new List<ContentBlockParam>
              {
                  new ContentBlockParam(new ToolResultBlockParam()
                  {
                      ToolUseID = toolUseBlock?.ID ?? "",
                      Content = $"Current temperature: {weatherData.temperature}°F"
                  })
              })}
          ]
      };

      var continuation = await client.Messages.Create(continuationParams);
      Console.WriteLine(continuation);
      ```

      ```go Go
      client := anthropic.NewClient()

      weatherTool := anthropic.ToolUnionParam{
      	OfTool: &anthropic.ToolParam{
      		Name:        "get_weather",
      		Description: anthropic.String("Get current weather for a location"),
      		InputSchema: anthropic.ToolInputSchemaParam{
      			Properties: map[string]any{
      				"location": map[string]any{
      					"type":        "string",
      					"description": "City name",
      				},
      			},
      			Required: []string{"location"},
      		},
      	},
      }

      response, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
      	Model:     anthropic.ModelClaudeSonnet4_6,
      	MaxTokens: 16000,
      	Thinking:  anthropic.ThinkingConfigParamOfEnabled(10000),
      	Tools:     []anthropic.ToolUnionParam{weatherTool},
      	Messages: []anthropic.MessageParam{
      		anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather in Paris?")),
      	},
      })
      if err != nil {
      	log.Fatal(err)
      }

      var toolUseBlock anthropic.ToolUseBlock
      for _, block := range response.Content {
      	switch v := block.AsAny().(type) {
      	case anthropic.ToolUseBlock:
      		toolUseBlock = v
      	}
      }

      weatherData := map[string]int{"temperature": 88}

      continuation, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
      	Model:     anthropic.ModelClaudeSonnet4_6,
      	MaxTokens: 16000,
      	Thinking:  anthropic.ThinkingConfigParamOfEnabled(10000),
      	Tools:     []anthropic.ToolUnionParam{weatherTool},
      	Messages: []anthropic.MessageParam{
      		anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather in Paris?")),
      		response.ToParam(),
      		anthropic.NewUserMessage(
      			anthropic.NewToolResultBlock(toolUseBlock.ID, fmt.Sprintf("Current temperature: %d°F", weatherData["temperature"]), false),
      		),
      	},
      })
      if err != nil {
      	log.Fatal(err)
      }

      fmt.Println(continuation)
      ```

      ```java Java
      import com.anthropic.models.messages.ThinkingBlock;
      import com.anthropic.models.messages.ThinkingBlockParam;
      // ...
      void main() {
          AnthropicClient client = AnthropicOkHttpClient.fromEnv();

          Tool weatherTool = Tool.builder()
              .name("get_weather")
              .description("Get current weather for a location")
              .inputSchema(Tool.InputSchema.builder()
                  .properties(JsonValue.from(Map.of(
                      "location", Map.of("type", "string", "description", "City name")
                  )))
                  .required(List.of("location"))
                  .build())
              .build();

          MessageCreateParams initialParams = MessageCreateParams.builder()
              .model(Model.CLAUDE_SONNET_4_6)
              .maxTokens(16000L)
              .enabledThinking(10000L)
              .addTool(weatherTool)
              .addUserMessage("What's the weather in Paris?")
              .build();

          Message response = client.messages().create(initialParams);

          ThinkingBlock thinkingBlock = null;
          ToolUseBlock toolUseBlock = null;
          for (var block : response.content()) {
              if (block.thinking().isPresent()) {
                  thinkingBlock = block.thinking().get();
              }
              if (block.toolUse().isPresent()) {
                  toolUseBlock = block.toolUse().get();
              }
          }

          int temperature = 88;

          // Second request: pass back thinking block and tool result
          MessageCreateParams continuationParams = MessageCreateParams.builder()
              .model(Model.CLAUDE_SONNET_4_6)
              .maxTokens(16000L)
              .enabledThinking(10000L)
              .addTool(weatherTool)
              .addUserMessage("What's the weather in Paris?")
              .addAssistantMessageOfBlockParams(List.of(
                  ContentBlockParam.ofThinking(ThinkingBlockParam.builder()
                      .thinking(thinkingBlock.thinking())
                      .signature(thinkingBlock.signature())
                      .build()),
                  ContentBlockParam.ofToolUse(ToolUseBlockParam.builder()
                      .id(toolUseBlock.id())
                      .name(toolUseBlock.name())
                      .input(toolUseBlock._input())
                      .build())
              ))
              .addUserMessageOfBlockParams(List.of(
                  ContentBlockParam.ofToolResult(
                      ToolResultBlockParam.builder()
                          .toolUseId(toolUseBlock.id())
                          .content("Current temperature: " + temperature + "°F")
                          .build()
                  )
              ))
              .build();

          Message continuation = client.messages().create(continuationParams);
          IO.println(continuation);
      }
      ```

      ```php PHP
      $client = new Client();

      $weatherTool = [
          'name' => 'get_weather',
          'description' => 'Get current weather for a location',
          'input_schema' => [
              'type' => 'object',
              'properties' => [
                  'location' => [
                      'type' => 'string',
                      'description' => 'City name'
                  ]
              ],
              'required' => ['location']
          ]
      ];

      $response = $client->messages->create(
          maxTokens: 16000,
          messages: [
              ['role' => 'user', 'content' => "What's the weather in Paris?"]
          ],
          model: 'claude-sonnet-4-6',
          thinking: ['type' => 'enabled', 'budget_tokens' => 10000],
          tools: [$weatherTool],
      );

      $thinkingBlock = null;
      $toolUseBlock = null;
      foreach ($response->content as $block) {
          if ($block->type === 'thinking') {
              $thinkingBlock = $block;
          }
          if ($block->type === 'tool_use') {
              $toolUseBlock = $block;
          }
      }

      $weatherData = ['temperature' => 88];

      $continuation = $client->messages->create(
          maxTokens: 16000,
          messages: [
              ['role' => 'user', 'content' => "What's the weather in Paris?"],
              ['role' => 'assistant', 'content' => [$thinkingBlock, $toolUseBlock]],
              ['role' => 'user', 'content' => [
                  [
                      'type' => 'tool_result',
                      'tool_use_id' => $toolUseBlock->id,
                      'content' => "Current temperature: {$weatherData['temperature']}°F"
                  ]
              ]]
          ],
          model: 'claude-sonnet-4-6',
          thinking: ['type' => 'enabled', 'budget_tokens' => 10000],
          tools: [$weatherTool],
      );

      echo $continuation;
      ```

      ```ruby Ruby
      client = Anthropic::Client.new

      weather_tool = {
        name: "get_weather",
        description: "Get current weather for a location",
        input_schema: {
          type: "object",
          properties: {
            location: { type: "string", description: "City name" }
          },
          required: ["location"]
        }
      }

      response = client.messages.create(
        model: "claude-sonnet-4-6",
        max_tokens: 16000,
        thinking: {
          type: "enabled",
          budget_tokens: 10000
        },
        tools: [weather_tool],
        messages: [
          { role: "user", content: "What's the weather in Paris?" }
        ]
      )

      thinking_block = response.content.find { |block| block.type == :thinking }
      tool_use_block = response.content.find { |block| block.type == :tool_use }

      raise "No tool_use block found" unless tool_use_block

      weather_data = { temperature: 88 }

      continuation = client.messages.create(
        model: "claude-sonnet-4-6",
        max_tokens: 16000,
        thinking: {
          type: "enabled",
          budget_tokens: 10000
        },
        tools: [weather_tool],
        messages: [
          { role: "user", content: "What's the weather in Paris?" },
          { role: "assistant", content: [thinking_block, tool_use_block] },
          { role: "user", content: [
            {
              type: "tool_result",
              tool_use_id: tool_use_block.id,
              content: "Current temperature: #{weather_data[:temperature]}°F"
            }
          ] }
        ]
      )

      puts continuation
      ```
    </CodeGroup>

    The API response now includes **only** text

    ```json Output
    {
      "content": [
        {
          "type": "text",
          "text": "Currently in Paris, the temperature is 88°F (31°C)"
        }
      ]
    }
    ```
  </Accordion>
</AccordionGroup>

### Preserving thinking blocks

During tool use, you must pass `thinking` blocks back to the API, and you must include the complete unmodified block back to the API. This is critical for maintaining the model's reasoning flow and conversation integrity.

<Tip>
  While you can omit `thinking` blocks from prior `assistant` role turns, always pass back all thinking blocks to the API for any multi-turn conversation. The API:

  * Automatically filters the provided thinking blocks
  * Uses the relevant thinking blocks necessary to preserve the model's reasoning
  * Only bills for the input tokens for the blocks shown to Claude

  Which blocks are kept depends on the model. See [Thinking block preservation by model](#thinking-block-preservation-in-claude-opus-45-and-later) for the per-class defaults. To override the default, use the [`clear_thinking_20251015` context-editing strategy](/docs/en/build-with-claude/context-editing#thinking-block-clearing).
</Tip>

<Note>
  When toggling thinking modes during a conversation, remember that the entire assistant turn (including tool use loops) must operate in a single thinking mode. For more details, see [Toggling thinking modes in conversations](#toggling-thinking-modes-in-conversations).
</Note>

When Claude invokes tools, it is pausing its construction of a response to await external information. When tool results are returned, Claude continues building that existing response. This necessitates preserving thinking blocks during tool use, for a couple of reasons:

1. **Reasoning continuity**: The thinking blocks capture Claude's step-by-step reasoning that led to tool requests. When you post tool results, including the original thinking ensures Claude can continue its reasoning from where it left off.

2. **Context maintenance**: While tool results appear as user messages in the API structure, they're part of a continuous reasoning flow. Preserving thinking blocks maintains this conceptual flow across multiple API calls. For more information on context management, see the [guide on context windows](/docs/en/build-with-claude/context-windows).

**Important**: When providing `thinking` blocks, the entire sequence of consecutive `thinking` blocks must match the outputs generated by the model during the original request; you can't rearrange or modify the sequence of these blocks.

<Note>
  If thinking blocks are modified, the API returns a 400 `invalid_request_error` whose message contains `` `thinking` or `redacted_thinking` blocks in the latest assistant message cannot be modified ``. The most common cause is application code that filters content blocks by type and drops `redacted_thinking` blocks, or that rebuilds the assistant message instead of echoing it. See [Thinking blocks cannot be modified](/docs/en/api/errors#thinking-blocks-cannot-be-modified) for the full error and fix steps.
</Note>

### Interleaved thinking

Extended thinking with tool use in Claude 4 models supports interleaved thinking, which enables Claude to think between tool calls and make more sophisticated reasoning after receiving tool results.

With interleaved thinking, Claude can:

* Reason about the results of a tool call before deciding what to do next
* Chain multiple tool calls with reasoning steps in between
* Make more nuanced decisions based on intermediate results

How you enable interleaved thinking depends on the model:

| Model                                                    | Interleaved thinking                                                                                                                                                              |
| -------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Claude Fable 5 Claude Mythos 5                           | Automatic with [adaptive thinking](/docs/en/build-with-claude/adaptive-thinking). Inter-tool reasoning moves into thinking blocks. No beta header needed.                         |
| [Claude Mythos Preview](https://anthropic.com/glasswing) | Automatic. Every inter-tool reasoning step moves into a thinking block instead of plain text. No beta header needed or supported.                                                 |
| Claude Opus 4.8                                          | Automatic with [adaptive thinking](/docs/en/build-with-claude/adaptive-thinking) (the only supported thinking mode). No beta header needed.                                       |
| Claude Opus 4.7                                          | Automatic with [adaptive thinking](/docs/en/build-with-claude/adaptive-thinking) (the only supported thinking mode). No beta header needed.                                       |
| Claude Opus 4.6                                          | Automatic with [adaptive thinking](/docs/en/build-with-claude/adaptive-thinking). The `interleaved-thinking-2025-05-14` beta header is deprecated and safely ignored if included. |
| Claude Sonnet 5                                          | Automatic with [adaptive thinking](/docs/en/build-with-claude/adaptive-thinking). The `interleaved-thinking-2025-05-14` beta header is deprecated and safely ignored if included. |
| Claude Sonnet 4.6                                        | Automatic with [adaptive thinking](/docs/en/build-with-claude/adaptive-thinking) (recommended). The beta header with manual `type: "enabled"` is still functional but deprecated. |
| Claude Opus 4.5                                          | Add the `interleaved-thinking-2025-05-14` [beta header](/docs/en/api/beta-headers) to your API request.                                                                           |
| Claude Haiku 4.5                                         | Not supported. The beta header is accepted on the Claude API but ignored.                                                                                                         |
| Earlier Claude 4 models                                  | Add the `interleaved-thinking-2025-05-14` [beta header](/docs/en/api/beta-headers) to your API request.                                                                           |

Earlier Claude 4 models here means Claude Sonnet 4.5, Claude Opus 4.1 (deprecated), Claude Opus 4 ([retired, except on Google Cloud](/docs/en/about-claude/model-deprecations)), and Claude Sonnet 4 ([retired, except on Bedrock and Google Cloud](/docs/en/about-claude/model-deprecations)).

Here are some important considerations for interleaved thinking:

* With interleaved thinking, the `budget_tokens` can exceed the `max_tokens` parameter, as it represents the total budget across all thinking blocks within one assistant turn.
* Interleaved thinking is only supported for [tools used via the Messages API](/docs/en/agents-and-tools/tool-use/overview).
* The Claude API and [Claude Platform on AWS](/docs/en/build-with-claude/claude-platform-on-aws) accept `interleaved-thinking-2025-05-14` in requests to any model without returning an error. On models that don't support interleaved thinking, the header is ignored. On Claude Opus 4.8, Claude Opus 4.7, Claude Opus 4.6, and Claude Sonnet 5, it's deprecated and safely ignored. On Claude Mythos Preview, it's not needed and safely ignored.
* On partner-operated platforms (for example, [Amazon Bedrock](/docs/en/build-with-claude/claude-in-amazon-bedrock) and [Google Cloud](/docs/en/build-with-claude/claude-on-vertex-ai)), if you pass `interleaved-thinking-2025-05-14` to any model aside from Claude Opus 4.8, Claude Opus 4.7, Claude Sonnet 5, Claude Opus 4.6, Claude Sonnet 4.6, Claude Opus 4.5, Claude Opus 4.1 (deprecated), Opus 4 ([retired, except on Google Cloud](/docs/en/about-claude/model-deprecations)), Sonnet 4.5, or Sonnet 4 ([retired, except on Bedrock and Google Cloud](/docs/en/about-claude/model-deprecations)), your request will fail.

<AccordionGroup>
  <Accordion title="Tool use without interleaved thinking">
    Without interleaved thinking, Claude thinks once at the start of the assistant turn. Subsequent responses after tool results continue without new thinking blocks.

    ```text
    User: "What's the total revenue if we sold 150 units at $50 each,
           and how does this compare to our average monthly revenue?"

    Turn 1: [thinking] "I need to calculate 150 * $50, then check the database..."
            [tool_use: calculator] { "expression": "150 * 50" }
      ↓ tool result: "7500"

    Turn 2: [tool_use: database_query] { "query": "SELECT AVG(revenue)..." }
            ↑ no thinking block
      ↓ tool result: "5200"

    Turn 3: [text] "The total revenue is $7,500, which is 44% above your
            average monthly revenue of $5,200."
            ↑ no thinking block
    ```
  </Accordion>

  <Accordion title="Tool use with interleaved thinking">
    With interleaved thinking enabled, Claude can think after receiving each tool result, allowing it to reason about intermediate results before continuing.

    ```text
    User: "What's the total revenue if we sold 150 units at $50 each,
           and how does this compare to our average monthly revenue?"

    Turn 1: [thinking] "I need to calculate 150 * $50 first..."
            [tool_use: calculator] { "expression": "150 * 50" }
      ↓ tool result: "7500"

    Turn 2: [thinking] "Got $7,500. Now I should query the database to compare..."
            [tool_use: database_query] { "query": "SELECT AVG(revenue)..." }
            ↑ thinking after receiving calculator result
      ↓ tool result: "5200"

    Turn 3: [thinking] "$7,500 vs $5,200 average - that's a 44% increase..."
            [text] "The total revenue is $7,500, which is 44% above your
            average monthly revenue of $5,200."
            ↑ thinking before final answer
    ```
  </Accordion>
</AccordionGroup>

## Extended thinking with prompt caching

[Prompt caching](/docs/en/build-with-claude/prompt-caching) with thinking has several important considerations:

<Tip>
  Extended thinking tasks often take longer than 5 minutes to complete. Consider using the [1-hour cache duration](/docs/en/build-with-claude/prompt-caching#1-hour-cache-duration) to maintain cache hits across longer thinking sessions and multi-step workflows.
</Tip>

**Thinking block context removal**

* On earlier Opus/Sonnet models and all Haiku models, thinking blocks from previous turns are removed from context, which can affect cache breakpoints. On Opus 4.5+ and Sonnet 4.6+, they are kept by default.
* When continuing conversations with tool use, thinking blocks are cached and count as input tokens when read from cache
* This creates a tradeoff: while thinking blocks don't consume context window space visually, they still count toward your input token usage when cached
* If thinking becomes disabled and you pass thinking content in the current tool use turn, the thinking content will be stripped and thinking will remain disabled for that request

**Cache invalidation patterns**

* Changes to thinking parameters (enabled/disabled or budget allocation) invalidate message cache breakpoints
* [Interleaved thinking](#interleaved-thinking) amplifies cache invalidation, as thinking blocks can occur between multiple [tool calls](#extended-thinking-with-tool-use)
* System prompts and tools remain cached despite thinking parameter changes or block removal

<Note>
  On earlier Opus/Sonnet models and all Haiku models, thinking blocks are removed for caching and context calculations; on Opus 4.5+ and Sonnet 4.6+, they are kept by default. In either case, they must be preserved when continuing conversations with [tool use](#extended-thinking-with-tool-use), especially with [interleaved thinking](#interleaved-thinking).
</Note>

### Understanding thinking block caching behavior

When using extended thinking with tool use, thinking blocks exhibit specific caching behavior that affects token counting:

**How it works:**

1. Caching only occurs when you make a subsequent request that includes tool results
2. When the subsequent request is made, the previous conversation history (including thinking blocks) can be cached
3. These cached thinking blocks count as input tokens in your usage metrics when read from the cache
4. When a non-tool-result user block is included: on Opus 4.5+ and Sonnet 4.6+, previous thinking blocks are kept; on earlier Opus/Sonnet models and all Haiku models, all previous thinking blocks are ignored and stripped from context

**Detailed example flow:**

**Request 1:**

```text wrap
User: "What's the weather in Paris?"
```

**Response 1:**

```text wrap
[thinking_block_1] + [tool_use block 1]
```

**Request 2:**

```text wrap
User: ["What's the weather in Paris?"],
Assistant: [thinking_block_1] + [tool_use block 1],
User: [tool_result_1, cache=True]
```

**Response 2:**

```text wrap
[thinking_block_2] + [text block 2]
```

Request 2 writes a cache of the request content (not the response). The cache includes the original user message, the first thinking block, tool use block, and the tool result.

**Request 3:**

```text wrap
User: ["What's the weather in Paris?"],
Assistant: [thinking_block_1] + [tool_use block 1],
User: [tool_result_1, cache=True],
Assistant: [thinking_block_2] + [text block 2],
User: [Text response, cache=True]
```

For Opus 4.5+ and Sonnet 4.6+, all previous thinking blocks are kept by default. For earlier Opus/Sonnet models and all Haiku models, because a non-tool-result user block was included, all previous thinking blocks are ignored and stripped from context. This request will be processed the same as:

```text wrap
User: ["What's the weather in Paris?"],
Assistant: [tool_use block 1],
User: [tool_result_1, cache=True],
Assistant: [text block 2],
User: [Text response, cache=True]
```

**Key points:**

* This caching behavior happens automatically, even without explicit `cache_control` markers
* This behavior is consistent whether using regular thinking or interleaved thinking

<AccordionGroup>
  <Accordion title="System prompt caching (preserved when thinking changes)">
    <CodeGroup>
      ```bash CLI
      # Fetch ~10 kB of Pride and Prejudice for the cached system block
      curl -s https://www.gutenberg.org/cache/epub/1342/pg1342.txt \
        | head -c 10000 > pride.txt

      # Emit a request body for the given thinking budget. Once CONTENT1
      # is populated (after the first turn), the assistant reply and a
      # follow-up user message are appended so the conversation grows.
      build_body() {
        cat <<YAML
      model: claude-sonnet-4-6
      max_tokens: 20000
      thinking:
        type: enabled
        budget_tokens: $1
      system:
        - type: text
          text: >-
            You are an AI assistant that is tasked with literary analysis.
            Analyze the following text carefully.
        - type: text
          text: "@./pride.txt"
          cache_control:
            type: ephemeral
      messages:
        - role: user
          content: Analyze the tone of this passage.
      YAML
        if [[ -n "${CONTENT1:-}" ]]; then
          printf '  - role: assistant\n    content: %s\n' "$CONTENT1"
          printf '  - role: user\n'
          printf '    content: Analyze the characters in this passage.\n'
        fi
      }

      # First request (budget 4000): establishes the cache. Capture usage
      # and content as two jsonl lines so the reply can be fed forward.
      printf 'First request - establishing cache\n'
      {
        read -r USAGE1
        read -r CONTENT1
      } < <(build_body 4000 \
        | ant messages create --transform '[usage,content]' --format jsonl)
      printf 'First response usage: %s\n' "$USAGE1"

      # Second request: same budget, system-prompt cache hit expected.
      printf '\nSecond request - same thinking parameters (cache hit expected)\n'
      USAGE2=$(build_body 4000 \
        | ant messages create --transform usage --format jsonl)
      printf 'Second response usage: %s\n' "$USAGE2"

      # Third request: budget changed to 8000. The cached system prompt
      # still hits; only message-block caching is invalidated.
      printf '\nThird request - different thinking parameters (cache miss for messages)\n'
      USAGE3=$(build_body 8000 \
        | ant messages create --transform usage --format jsonl)
      printf 'Third response usage: %s\n' "$USAGE3"
      ```

      ```python Python
      import requests
      from bs4 import BeautifulSoup

      client = Anthropic()


      def fetch_article_content(url):
          response = requests.get(url)
          soup = BeautifulSoup(response.content, "html.parser")

          # Remove script and style elements
          for script in soup(["script", "style"]):
              script.decompose()

          # Get text
          text = soup.get_text()

          # Break into lines and remove leading and trailing space on each
          lines = (line.strip() for line in text.splitlines())
          # Break multi-headlines into a line each
          chunks = (phrase.strip() for line in lines for phrase in line.split("  "))
          # Drop blank lines
          text = "\n".join(chunk for chunk in chunks if chunk)

          return text


      # Fetch the content of the article
      book_url = "https://www.gutenberg.org/cache/epub/1342/pg1342.txt"
      book_content = fetch_article_content(book_url)
      # Use just enough text for caching (first few chapters)
      LARGE_TEXT = book_content[:10000]

      SYSTEM_PROMPT = [
          {
              "type": "text",
              "text": "You are an AI assistant that is tasked with literary analysis. Analyze the following text carefully.",
          },
          {"type": "text", "text": LARGE_TEXT, "cache_control": {"type": "ephemeral"}},
      ]

      MESSAGES = [{"role": "user", "content": "Analyze the tone of this passage."}]

      # First request - establish cache
      print("First request - establishing cache")
      response1 = client.messages.create(
          model="claude-sonnet-4-6",
          max_tokens=20000,
          thinking={"type": "enabled", "budget_tokens": 4000},
          system=SYSTEM_PROMPT,
          messages=MESSAGES,
      )

      print(f"First response usage: {response1.usage}")

      MESSAGES.append({"role": "assistant", "content": response1.content})
      MESSAGES.append({"role": "user", "content": "Analyze the characters in this passage."})
      # Second request - same thinking parameters (cache hit expected)
      print("\nSecond request - same thinking parameters (cache hit expected)")
      response2 = client.messages.create(
          model="claude-sonnet-4-6",
          max_tokens=20000,
          thinking={"type": "enabled", "budget_tokens": 4000},
          system=SYSTEM_PROMPT,
          messages=MESSAGES,
      )

      print(f"Second response usage: {response2.usage}")

      # Third request - different thinking parameters (cache miss for messages)
      print("\nThird request - different thinking parameters (cache miss for messages)")
      response3 = client.messages.create(
          model="claude-sonnet-4-6",
          max_tokens=20000,
          thinking={
              "type": "enabled",
              "budget_tokens": 8000,  # Changed thinking budget
          },
          system=SYSTEM_PROMPT,  # System prompt remains cached
          messages=MESSAGES,  # Messages cache is invalidated
      )

      print(f"Third response usage: {response3.usage}")
      ```

      ```typescript TypeScript
      import axios from "axios";
      import * as cheerio from "cheerio";

      const client = new Anthropic();

      async function fetchArticleContent(url: string): Promise<string> {
        const response = await axios.get(url);
        const $ = cheerio.load(response.data);
        $("script, style").remove();
        let text = $.text();
        const lines = text.split("\n").map((line) => line.trim());
        text = lines.filter((line) => line.length > 0).join("\n");
        return text;
      }

      const bookUrl = "https://www.gutenberg.org/cache/epub/1342/pg1342.txt";
      const bookContent = await fetchArticleContent(bookUrl);
      const LARGE_TEXT = bookContent.slice(0, 10000);

      const SYSTEM_PROMPT: Anthropic.TextBlockParam[] = [
        {
          type: "text",
          text: "You are an AI assistant that is tasked with literary analysis. Analyze the following text carefully."
        },
        {
          type: "text",
          text: LARGE_TEXT,
          cache_control: { type: "ephemeral" }
        }
      ];

      const messages: Anthropic.MessageParam[] = [
        { role: "user", content: "Analyze the tone of this passage." }
      ];

      // First request - establish cache
      console.log("First request - establishing cache");
      const response1 = await client.messages.create({
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: { type: "enabled", budget_tokens: 4000 },
        system: SYSTEM_PROMPT,
        messages
      });

      console.log(`First response usage: ${JSON.stringify(response1.usage)}`);

      messages.push({
        role: "assistant",
        content: response1.content
      });
      messages.push({
        role: "user",
        content: "Analyze the characters in this passage."
      });

      // Second request - same thinking parameters (cache hit expected)
      console.log("\nSecond request - same thinking parameters (cache hit expected)");
      const response2 = await client.messages.create({
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: { type: "enabled", budget_tokens: 4000 },
        system: SYSTEM_PROMPT,
        messages
      });

      console.log(`Second response usage: ${JSON.stringify(response2.usage)}`);

      // Third request - different thinking parameters (cache miss for messages)
      console.log("\nThird request - different thinking parameters (cache miss for messages)");
      const response3 = await client.messages.create({
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: { type: "enabled", budget_tokens: 8000 },
        system: SYSTEM_PROMPT,
        messages
      });

      console.log(`Third response usage: ${JSON.stringify(response3.usage)}`);
      ```

      ```csharp C#
      AnthropicClient client = new();

      // Fetch book content
      using var httpClient = new HttpClient();
      var bookContent = await httpClient.GetStringAsync("https://www.gutenberg.org/cache/epub/1342/pg1342.txt");
      var largeText = bookContent.Substring(0, Math.Min(10000, bookContent.Length));

      var systemPrompt = new MessageCreateParamsSystem(new List<TextBlockParam>
      {
          new TextBlockParam()
          {
              Text = "You are an AI assistant that is tasked with literary analysis. Analyze the following text carefully."
          },
          new TextBlockParam()
          {
              Text = largeText,
              CacheControl = new CacheControlEphemeral(),
          },
      });

      var messages = new List<MessageParam>
      {
          new() { Role = Role.User, Content = "Analyze the tone of this passage." }
      };

      // First request - establish cache
      Console.WriteLine("First request - establishing cache");
      var parameters1 = new MessageCreateParams
      {
          Model = Model.ClaudeSonnet4_6,
          MaxTokens = 20000,
          Thinking = new ThinkingConfigEnabled(budgetTokens: 4000),
          System = systemPrompt,
          Messages = messages
      };

      var response1 = await client.Messages.Create(parameters1);
      Console.WriteLine($"First response usage: {response1.Usage}");

      messages.Add(new() { Role = Role.Assistant, Content = response1.Content.Select(block => new ContentBlockParam(block.Json)).ToList() });
      messages.Add(new() { Role = Role.User, Content = "Analyze the characters in this passage." });

      // Second request - same thinking parameters (cache hit expected)
      Console.WriteLine("\nSecond request - same thinking parameters (cache hit expected)");
      var parameters2 = new MessageCreateParams
      {
          Model = Model.ClaudeSonnet4_6,
          MaxTokens = 20000,
          Thinking = new ThinkingConfigEnabled(budgetTokens: 4000),
          System = systemPrompt,
          Messages = messages
      };

      var response2 = await client.Messages.Create(parameters2);
      Console.WriteLine($"Second response usage: {response2.Usage}");

      // Third request - different thinking parameters (cache miss for messages)
      Console.WriteLine("\nThird request - different thinking parameters (cache miss for messages)");
      var parameters3 = new MessageCreateParams
      {
          Model = Model.ClaudeSonnet4_6,
          MaxTokens = 20000,
          Thinking = new ThinkingConfigEnabled(budgetTokens: 8000),
          System = systemPrompt,
          Messages = messages
      };

      var response3 = await client.Messages.Create(parameters3);
      Console.WriteLine($"Third response usage: {response3.Usage}");
      ```

      ```go Go
      client := anthropic.NewClient()

      // Fetch book content
      resp, err := http.Get("https://www.gutenberg.org/cache/epub/1342/pg1342.txt")
      if err != nil {
      	log.Fatal(err)
      }
      defer resp.Body.Close()

      body, err := io.ReadAll(resp.Body)
      if err != nil {
      	log.Fatal(err)
      }

      largeText := string(body)
      if len(largeText) > 10000 {
      	largeText = largeText[:10000]
      }

      systemPrompt := []anthropic.TextBlockParam{
      	{Text: "You are an AI assistant that is tasked with literary analysis. Analyze the following text carefully."},
      	{
      		Text:         largeText,
      		CacheControl: anthropic.NewCacheControlEphemeralParam(),
      	},
      }

      messages := []anthropic.MessageParam{
      	anthropic.NewUserMessage(anthropic.NewTextBlock("Analyze the tone of this passage.")),
      }

      // First request - establish cache
      fmt.Println("First request - establishing cache")
      response1, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
      	Model:     anthropic.ModelClaudeSonnet4_6,
      	MaxTokens: 20000,
      	Thinking:  anthropic.ThinkingConfigParamOfEnabled(4000),
      	System:    systemPrompt,
      	Messages:  messages,
      })
      if err != nil {
      	log.Fatal(err)
      }

      fmt.Printf("First response usage: %s\n", response1.Usage.RawJSON())

      messages = append(messages, response1.ToParam())
      messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("Analyze the characters in this passage.")))

      // Second request - same thinking parameters (cache hit expected)
      fmt.Println("\nSecond request - same thinking parameters (cache hit expected)")
      response2, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
      	Model:     anthropic.ModelClaudeSonnet4_6,
      	MaxTokens: 20000,
      	Thinking:  anthropic.ThinkingConfigParamOfEnabled(4000),
      	System:    systemPrompt,
      	Messages:  messages,
      })
      if err != nil {
      	log.Fatal(err)
      }

      fmt.Printf("Second response usage: %s\n", response2.Usage.RawJSON())

      // Third request - different thinking parameters (cache miss for messages)
      fmt.Println("\nThird request - different thinking parameters (cache miss for messages)")
      response3, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
      	Model:     anthropic.ModelClaudeSonnet4_6,
      	MaxTokens: 20000,
      	Thinking:  anthropic.ThinkingConfigParamOfEnabled(8000),
      	System:    systemPrompt,
      	Messages:  messages,
      })
      if err != nil {
      	log.Fatal(err)
      }

      fmt.Printf("Third response usage: %s\n", response3.Usage.RawJSON())
      ```

      ```java Java
      import com.anthropic.models.messages.CacheControlEphemeral;
      // ...
      void main() throws Exception {
          AnthropicClient client = AnthropicOkHttpClient.fromEnv();

          // Fetch book content
          HttpClient httpClient = HttpClient.newHttpClient();
          HttpRequest request = HttpRequest.newBuilder()
              .uri(URI.create("https://www.gutenberg.org/cache/epub/1342/pg1342.txt"))
              .build();
          HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
          String bookContent = response.body();
          String largeText = bookContent.substring(0, Math.min(10000, bookContent.length()));

          List<TextBlockParam> systemPrompt = List.of(
              TextBlockParam.builder()
                  .text("You are an AI assistant that is tasked with literary analysis. Analyze the following text carefully.")
                  .build(),
              TextBlockParam.builder()
                  .text(largeText)
                  .cacheControl(CacheControlEphemeral.builder().build())
                  .build()
          );

          // First request - establish cache
          IO.println("First request - establishing cache");
          MessageCreateParams params1 = MessageCreateParams.builder()
              .model(Model.CLAUDE_SONNET_4_6)
              .maxTokens(20000L)
              .enabledThinking(4000L)
              .systemOfTextBlockParams(systemPrompt)
              .addUserMessage("Analyze the tone of this passage.")
              .build();

          Message response1 = client.messages().create(params1);
          IO.println("First response usage: " + response1.usage());

          // Second request - same thinking parameters (cache hit expected)
          IO.println("\nSecond request - same thinking parameters (cache hit expected)");
          MessageCreateParams params2 = MessageCreateParams.builder()
              .model(Model.CLAUDE_SONNET_4_6)
              .maxTokens(20000L)
              .enabledThinking(4000L)
              .systemOfTextBlockParams(systemPrompt)
              .addUserMessage("Analyze the tone of this passage.")
              .addAssistantMessageOfBlockParams(response1.content().stream()
                  .map(block -> block.toParam())
                  .collect(java.util.stream.Collectors.toList()))
              .addUserMessage("Analyze the characters in this passage.")
              .build();

          Message response2 = client.messages().create(params2);
          IO.println("Second response usage: " + response2.usage());

          // Third request - different thinking parameters (cache miss for messages)
          IO.println("\nThird request - different thinking parameters (cache miss for messages)");
          MessageCreateParams params3 = MessageCreateParams.builder()
              .model(Model.CLAUDE_SONNET_4_6)
              .maxTokens(20000L)
              .enabledThinking(8000L)
              .systemOfTextBlockParams(systemPrompt)
              .addUserMessage("Analyze the tone of this passage.")
              .addAssistantMessageOfBlockParams(response1.content().stream()
                  .map(block -> block.toParam())
                  .collect(java.util.stream.Collectors.toList()))
              .addUserMessage("Analyze the characters in this passage.")
              .build();

          Message response3 = client.messages().create(params3);
          IO.println("Third response usage: " + response3.usage());
      }
      ```

      ```php PHP
      $client = new Client();

      // Fetch book content
      $bookContent = file_get_contents("https://www.gutenberg.org/cache/epub/1342/pg1342.txt");
      $largeText = substr($bookContent, 0, 10000);

      $systemPrompt = [
          [
              'type' => 'text',
              'text' => 'You are an AI assistant that is tasked with literary analysis. Analyze the following text carefully.'
          ],
          [
              'type' => 'text',
              'text' => $largeText,
              'cache_control' => ['type' => 'ephemeral']
          ]
      ];

      $messages = [
          ['role' => 'user', 'content' => 'Analyze the tone of this passage.']
      ];

      // First request - establish cache
      echo "First request - establishing cache\n";
      $response1 = $client->messages->create(
          maxTokens: 20000,
          messages: $messages,
          model: 'claude-sonnet-4-6',
          system: $systemPrompt,
          thinking: ['type' => 'enabled', 'budget_tokens' => 4000],
      );

      echo "First response usage: " . json_encode($response1->usage) . "\n";

      $messages[] = ['role' => 'assistant', 'content' => $response1->content];
      $messages[] = ['role' => 'user', 'content' => 'Analyze the characters in this passage.'];

      // Second request - same thinking parameters (cache hit expected)
      echo "\nSecond request - same thinking parameters (cache hit expected)\n";
      $response2 = $client->messages->create(
          maxTokens: 20000,
          messages: $messages,
          model: 'claude-sonnet-4-6',
          system: $systemPrompt,
          thinking: ['type' => 'enabled', 'budget_tokens' => 4000],
      );

      echo "Second response usage: " . json_encode($response2->usage) . "\n";

      // Third request - different thinking parameters (cache miss for messages)
      echo "\nThird request - different thinking parameters (cache miss for messages)\n";
      $response3 = $client->messages->create(
          maxTokens: 20000,
          messages: $messages,
          model: 'claude-sonnet-4-6',
          system: $systemPrompt,
          thinking: ['type' => 'enabled', 'budget_tokens' => 8000],
      );

      echo "Third response usage: " . json_encode($response3->usage) . "\n";
      ```

      ```ruby Ruby
      require "net/http"
      require "uri"

      client = Anthropic::Client.new

      # Fetch book content
      uri = URI("https://www.gutenberg.org/cache/epub/1342/pg1342.txt")
      response = Net::HTTP.get_response(uri)
      book_content = response.body
      large_text = book_content[0...10000]

      system_prompt = [
        {
          type: "text",
          text: "You are an AI assistant that is tasked with literary analysis. Analyze the following text carefully."
        },
        {
          type: "text",
          text: large_text,
          cache_control: { type: "ephemeral" }
        }
      ]

      messages = [
        { role: "user", content: "Analyze the tone of this passage." }
      ]

      # First request - establish cache
      puts "First request - establishing cache"
      response1 = client.messages.create(
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: {
          type: "enabled",
          budget_tokens: 4000
        },
        system: system_prompt,
        messages: messages
      )

      puts "First response usage: #{response1.usage}"

      messages << { role: "assistant", content: response1.content }
      messages << { role: "user", content: "Analyze the characters in this passage." }

      # Second request - same thinking parameters (cache hit expected)
      puts "\nSecond request - same thinking parameters (cache hit expected)"
      response2 = client.messages.create(
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: {
          type: "enabled",
          budget_tokens: 4000
        },
        system: system_prompt,
        messages: messages
      )

      puts "Second response usage: #{response2.usage}"

      # Third request - different thinking parameters (cache miss for messages)
      puts "\nThird request - different thinking parameters (cache miss for messages)"
      response3 = client.messages.create(
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: {
          type: "enabled",
          budget_tokens: 8000
        },
        system: system_prompt,
        messages: messages
      )

      puts "Third response usage: #{response3.usage}"
      ```
    </CodeGroup>
  </Accordion>

  <Accordion title="Messages caching (invalidated when thinking changes)">
    <CodeGroup>
      ```bash CLI
      # This workflow doesn't translate well to a one-off shell command.
      # See the SDK tabs for the multi-turn pattern; the per-turn CLI
      # invocation is identical to the earlier prompt-caching example.
      ```

      ```python Python
      import requests
      from bs4 import BeautifulSoup

      client = Anthropic()


      def fetch_article_content(url):
          response = requests.get(url)
          soup = BeautifulSoup(response.content, "html.parser")

          # Remove script and style elements
          for script in soup(["script", "style"]):
              script.decompose()

          # Get text
          text = soup.get_text()

          # Break into lines and remove leading and trailing space on each
          lines = (line.strip() for line in text.splitlines())
          # Break multi-headlines into a line each
          chunks = (phrase.strip() for line in lines for phrase in line.split("  "))
          # Drop blank lines
          text = "\n".join(chunk for chunk in chunks if chunk)

          return text


      # Fetch the content of the article
      book_url = "https://www.gutenberg.org/cache/epub/1342/pg1342.txt"
      book_content = fetch_article_content(book_url)
      # Use just enough text for caching (first few chapters)
      LARGE_TEXT = book_content[:10000]

      # No system prompt - caching in messages instead
      MESSAGES = [
          {
              "role": "user",
              "content": [
                  {
                      "type": "text",
                      "text": LARGE_TEXT,
                      "cache_control": {"type": "ephemeral"},
                  },
                  {"type": "text", "text": "Analyze the tone of this passage."},
              ],
          }
      ]

      # First request - establish cache
      print("First request - establishing cache")
      response1 = client.messages.create(
          model="claude-sonnet-4-6",
          max_tokens=20000,
          thinking={"type": "enabled", "budget_tokens": 4000},
          messages=MESSAGES,
      )

      print(f"First response usage: {response1.usage}")

      MESSAGES.append({"role": "assistant", "content": response1.content})
      MESSAGES.append({"role": "user", "content": "Analyze the characters in this passage."})
      # Second request - same thinking parameters (cache hit expected)
      print("\nSecond request - same thinking parameters (cache hit expected)")
      response2 = client.messages.create(
          model="claude-sonnet-4-6",
          max_tokens=20000,
          thinking={
              "type": "enabled",
              "budget_tokens": 4000,  # Same thinking budget
          },
          messages=MESSAGES,
      )

      print(f"Second response usage: {response2.usage}")

      MESSAGES.append({"role": "assistant", "content": response2.content})
      MESSAGES.append({"role": "user", "content": "Analyze the setting in this passage."})

      # Third request - different thinking budget (cache miss expected)
      print("\nThird request - different thinking budget (cache miss expected)")
      response3 = client.messages.create(
          model="claude-sonnet-4-6",
          max_tokens=20000,
          thinking={
              "type": "enabled",
              "budget_tokens": 8000,  # Different thinking budget breaks cache
          },
          messages=MESSAGES,
      )

      print(f"Third response usage: {response3.usage}")
      ```

      ```typescript TypeScript
      import axios from "axios";
      import * as cheerio from "cheerio";

      const client = new Anthropic();

      async function fetchArticleContent(url: string): Promise<string> {
        const response = await axios.get(url);
        const $ = cheerio.load(response.data);

        // Remove script and style elements
        $("script, style").remove();

        // Get text
        let text = $.text();

        // Clean up text (break into lines, remove whitespace)
        const lines = text.split("\n").map((line) => line.trim());
        const chunks = lines.flatMap((line) => line.split("  ").map((phrase) => phrase.trim()));
        text = chunks.filter((chunk) => chunk).join("\n");

        return text;
      }

      const bookUrl = "https://www.gutenberg.org/cache/epub/1342/pg1342.txt";
      const bookContent = await fetchArticleContent(bookUrl);
      const LARGE_TEXT = bookContent.substring(0, 10000);

      // No system prompt - caching in messages instead
      const messages: Anthropic.MessageParam[] = [
        {
          role: "user",
          content: [
            {
              type: "text",
              text: LARGE_TEXT,
              cache_control: { type: "ephemeral" }
            },
            {
              type: "text",
              text: "Analyze the tone of this passage."
            }
          ]
        }
      ];

      // First request - establish cache
      console.log("First request - establishing cache");
      const response1 = await client.messages.create({
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: { type: "enabled", budget_tokens: 4000 },
        messages
      });

      console.log("First response usage: ", response1.usage);

      messages.push(
        { role: "assistant", content: response1.content },
        { role: "user", content: "Analyze the characters in this passage." }
      );

      // Second request - same thinking parameters (cache hit expected)
      console.log("\nSecond request - same thinking parameters (cache hit expected)");
      const response2 = await client.messages.create({
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: { type: "enabled", budget_tokens: 4000 },
        messages
      });

      console.log("Second response usage: ", response2.usage);

      messages.push(
        { role: "assistant", content: response2.content },
        { role: "user", content: "Analyze the setting in this passage." }
      );

      // Third request - different thinking budget (cache miss expected)
      console.log("\nThird request - different thinking budget (cache miss expected)");
      const response3 = await client.messages.create({
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: { type: "enabled", budget_tokens: 8000 },
        messages
      });

      console.log("Third response usage: ", response3.usage);
      ```

      ```csharp C#
      AnthropicClient client = new();

      string bookUrl = "https://www.gutenberg.org/cache/epub/1342/pg1342.txt";
      string bookContent = await FetchArticleContent(bookUrl);
      string largeText = bookContent.Substring(0, Math.Min(10000, bookContent.Length));

      Console.WriteLine("First request - establishing cache");
      var parameters1 = new MessageCreateParams
      {
          Model = Model.ClaudeSonnet4_6,
          MaxTokens = 20000,
          Thinking = new ThinkingConfigEnabled(budgetTokens: 4000),
          Messages =
          [
              new()
              {
                  Role = Role.User,
                  Content = new MessageParamContent(new List<ContentBlockParam>
                  {
                      new ContentBlockParam(new TextBlockParam()
                      {
                          Text = largeText,
                          CacheControl = new CacheControlEphemeral(),
                      }),
                      new ContentBlockParam(new TextBlockParam()
                      {
                          Text = "Analyze the tone of this passage."
                      }),
                  })
              }
          ]
      };

      var response1 = await client.Messages.Create(parameters1);
      Console.WriteLine($"First response usage: {response1.Usage}");

      Console.WriteLine("\nSecond request - same thinking parameters (cache hit expected)");
      var parameters2 = new MessageCreateParams
      {
          Model = Model.ClaudeSonnet4_6,
          MaxTokens = 20000,
          Thinking = new ThinkingConfigEnabled(budgetTokens: 4000),
          Messages =
          [
              new()
              {
                  Role = Role.User,
                  Content = new MessageParamContent(new List<ContentBlockParam>
                  {
                      new ContentBlockParam(new TextBlockParam()
                      {
                          Text = largeText,
                          CacheControl = new CacheControlEphemeral(),
                      }),
                      new ContentBlockParam(new TextBlockParam()
                      {
                          Text = "Analyze the tone of this passage."
                      }),
                  })
              },
              new()
              {
                  Role = Role.Assistant,
                  Content = response1.Content.Select(block => new ContentBlockParam(block.Json)).ToList()
              },
              new()
              {
                  Role = Role.User,
                  Content = "Analyze the characters in this passage."
              }
          ]
      };

      var response2 = await client.Messages.Create(parameters2);
      Console.WriteLine($"Second response usage: {response2.Usage}");

      Console.WriteLine("\nThird request - different thinking budget (cache miss expected)");
      var parameters3 = new MessageCreateParams
      {
          Model = Model.ClaudeSonnet4_6,
          MaxTokens = 20000,
          Thinking = new ThinkingConfigEnabled(budgetTokens: 8000),
          Messages =
          [
              new()
              {
                  Role = Role.User,
                  Content = new MessageParamContent(new List<ContentBlockParam>
                  {
                      new ContentBlockParam(new TextBlockParam()
                      {
                          Text = largeText,
                          CacheControl = new CacheControlEphemeral(),
                      }),
                      new ContentBlockParam(new TextBlockParam()
                      {
                          Text = "Analyze the tone of this passage."
                      }),
                  })
              },
              new()
              {
                  Role = Role.Assistant,
                  Content = response1.Content.Select(block => new ContentBlockParam(block.Json)).ToList()
              },
              new()
              {
                  Role = Role.User,
                  Content = "Analyze the characters in this passage."
              },
              new()
              {
                  Role = Role.Assistant,
                  Content = response2.Content.Select(block => new ContentBlockParam(block.Json)).ToList()
              },
              new()
              {
                  Role = Role.User,
                  Content = "Analyze the setting in this passage."
              }
          ]
      };

      var response3 = await client.Messages.Create(parameters3);
      Console.WriteLine($"Third response usage: {response3.Usage}");

      static async Task<string> FetchArticleContent(string url)
      {
          using HttpClient httpClient = new();
          string content = await httpClient.GetStringAsync(url);
          return content;
      }
      ```

      ```go Go
      client := anthropic.NewClient()

      bookURL := "https://www.gutenberg.org/cache/epub/1342/pg1342.txt"
      bookContent, err := fetchArticleContent(bookURL)
      if err != nil {
      	log.Fatal(err)
      }

      largeText := bookContent
      if len(largeText) > 10000 {
      	largeText = largeText[:10000]
      }

      // No system prompt - caching in messages instead
      messages := []anthropic.MessageParam{
      	anthropic.NewUserMessage(
      		anthropic.ContentBlockParamUnion{OfText: &anthropic.TextBlockParam{
      			Text:         largeText,
      			CacheControl: anthropic.NewCacheControlEphemeralParam(),
      		}},
      		anthropic.NewTextBlock("Analyze the tone of this passage."),
      	),
      }

      // First request - establish cache
      fmt.Println("First request - establishing cache")
      response1, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
      	Model:     anthropic.ModelClaudeSonnet4_6,
      	MaxTokens: 20000,
      	Thinking:  anthropic.ThinkingConfigParamOfEnabled(4000),
      	Messages:  messages,
      })
      if err != nil {
      	log.Fatal(err)
      }
      fmt.Printf("First response usage: %s\n", response1.Usage.RawJSON())

      messages = append(messages, response1.ToParam())
      messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("Analyze the characters in this passage.")))

      // Second request - same thinking parameters (cache hit expected)
      fmt.Println("\nSecond request - same thinking parameters (cache hit expected)")
      response2, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
      	Model:     anthropic.ModelClaudeSonnet4_6,
      	MaxTokens: 20000,
      	Thinking:  anthropic.ThinkingConfigParamOfEnabled(4000),
      	Messages:  messages,
      })
      if err != nil {
      	log.Fatal(err)
      }
      fmt.Printf("Second response usage: %s\n", response2.Usage.RawJSON())

      messages = append(messages, response2.ToParam())
      messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock("Analyze the setting in this passage.")))

      // Third request - different thinking budget (cache miss expected)
      fmt.Println("\nThird request - different thinking budget (cache miss expected)")
      response3, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
      	Model:     anthropic.ModelClaudeSonnet4_6,
      	MaxTokens: 20000,
      	Thinking:  anthropic.ThinkingConfigParamOfEnabled(8000),
      	Messages:  messages,
      })
      if err != nil {
      	log.Fatal(err)
      }
      fmt.Printf("Third response usage: %s\n", response3.Usage.RawJSON())
      ```

      ```java Java
      import com.anthropic.models.messages.CacheControlEphemeral;
      // ...
      void main() throws Exception {
          AnthropicClient client = AnthropicOkHttpClient.fromEnv();

          String bookUrl = "https://www.gutenberg.org/cache/epub/1342/pg1342.txt";
          String bookContent = fetchArticleContent(bookUrl);
          String largeText = bookContent.substring(0, Math.min(10000, bookContent.length()));

          // First request - establishing cache
          IO.println("First request - establishing cache");
          MessageCreateParams params1 = MessageCreateParams.builder()
              .model(Model.CLAUDE_SONNET_4_6)
              .maxTokens(20000L)
              .enabledThinking(4000L)
              .addUserMessageOfBlockParams(List.of(
                  ContentBlockParam.ofText(TextBlockParam.builder()
                      .text(largeText)
                      .cacheControl(CacheControlEphemeral.builder().build())
                      .build()),
                  ContentBlockParam.ofText(TextBlockParam.builder()
                      .text("Analyze the tone of this passage.")
                      .build())
              ))
              .build();

          Message response1 = client.messages().create(params1);
          IO.println("First response usage: " + response1.usage());

          // Second request - same thinking parameters (cache hit expected)
          IO.println("\nSecond request - same thinking parameters (cache hit expected)");
          MessageCreateParams params2 = MessageCreateParams.builder()
              .model(Model.CLAUDE_SONNET_4_6)
              .maxTokens(20000L)
              .enabledThinking(4000L)
              .addUserMessageOfBlockParams(List.of(
                  ContentBlockParam.ofText(TextBlockParam.builder()
                      .text(largeText)
                      .cacheControl(CacheControlEphemeral.builder().build())
                      .build()),
                  ContentBlockParam.ofText(TextBlockParam.builder()
                      .text("Analyze the tone of this passage.")
                      .build())
              ))
              .addAssistantMessageOfBlockParams(response1.content().stream()
                  .map(block -> block.toParam())
                  .collect(java.util.stream.Collectors.toList()))
              .addUserMessage("Analyze the characters in this passage.")
              .build();

          Message response2 = client.messages().create(params2);
          IO.println("Second response usage: " + response2.usage());

          // Third request - different thinking budget (cache miss expected)
          IO.println("\nThird request - different thinking budget (cache miss expected)");
          MessageCreateParams params3 = MessageCreateParams.builder()
              .model(Model.CLAUDE_SONNET_4_6)
              .maxTokens(20000L)
              .enabledThinking(8000L)
              .addUserMessageOfBlockParams(List.of(
                  ContentBlockParam.ofText(TextBlockParam.builder()
                      .text(largeText)
                      .cacheControl(CacheControlEphemeral.builder().build())
                      .build()),
                  ContentBlockParam.ofText(TextBlockParam.builder()
                      .text("Analyze the tone of this passage.")
                      .build())
              ))
              .addAssistantMessageOfBlockParams(response1.content().stream()
                  .map(block -> block.toParam())
                  .collect(java.util.stream.Collectors.toList()))
              .addUserMessage("Analyze the characters in this passage.")
              .addAssistantMessageOfBlockParams(response2.content().stream()
                  .map(block -> block.toParam())
                  .collect(java.util.stream.Collectors.toList()))
              .addUserMessage("Analyze the setting in this passage.")
              .build();

          Message response3 = client.messages().create(params3);
          IO.println("Third response usage: " + response3.usage());
      }

      String fetchArticleContent(String url) throws Exception {
          HttpClient client = HttpClient.newHttpClient();
          HttpRequest request = HttpRequest.newBuilder()
              .uri(URI.create(url))
              .build();
          HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());
          return response.body();
      }
      ```

      ```php PHP
      function fetchArticleContent($url) {
          $content = file_get_contents($url);
          $lines = explode("\n", $content);
          $cleanedLines = array_filter(array_map('trim', $lines));
          return implode("\n", $cleanedLines);
      }

      $client = new Client();

      $bookUrl = "https://www.gutenberg.org/cache/epub/1342/pg1342.txt";
      $bookContent = fetchArticleContent($bookUrl);
      $largeText = substr($bookContent, 0, 10000);

      echo "First request - establishing cache\n";
      $response1 = $client->messages->create(
          maxTokens: 20000,
          messages: [[
              'role' => 'user',
              'content' => [
                  [
                      'type' => 'text',
                      'text' => $largeText,
                      'cache_control' => ['type' => 'ephemeral']
                  ],
                  [
                      'type' => 'text',
                      'text' => 'Analyze the tone of this passage.'
                  ]
              ]
          ]],
          model: 'claude-sonnet-4-6',
          thinking: ['type' => 'enabled', 'budget_tokens' => 4000],
      );

      echo "First response usage: " . json_encode($response1->usage) . "\n";

      echo "\nSecond request - same thinking parameters (cache hit expected)\n";
      $response2 = $client->messages->create(
          maxTokens: 20000,
          messages: [
              [
                  'role' => 'user',
                  'content' => [
                      [
                          'type' => 'text',
                          'text' => $largeText,
                          'cache_control' => ['type' => 'ephemeral']
                      ],
                      [
                          'type' => 'text',
                          'text' => 'Analyze the tone of this passage.'
                      ]
                  ]
              ],
              [
                  'role' => 'assistant',
                  'content' => $response1->content
              ],
              [
                  'role' => 'user',
                  'content' => 'Analyze the characters in this passage.'
              ]
          ],
          model: 'claude-sonnet-4-6',
          thinking: ['type' => 'enabled', 'budget_tokens' => 4000],
      );

      echo "Second response usage: " . json_encode($response2->usage) . "\n";

      echo "\nThird request - different thinking budget (cache miss expected)\n";
      $response3 = $client->messages->create(
          maxTokens: 20000,
          messages: [
              [
                  'role' => 'user',
                  'content' => [
                      [
                          'type' => 'text',
                          'text' => $largeText,
                          'cache_control' => ['type' => 'ephemeral']
                      ],
                      [
                          'type' => 'text',
                          'text' => 'Analyze the tone of this passage.'
                      ]
                  ]
              ],
              [
                  'role' => 'assistant',
                  'content' => $response1->content
              ],
              [
                  'role' => 'user',
                  'content' => 'Analyze the characters in this passage.'
              ],
              [
                  'role' => 'assistant',
                  'content' => $response2->content
              ],
              [
                  'role' => 'user',
                  'content' => 'Analyze the setting in this passage.'
              ]
          ],
          model: 'claude-sonnet-4-6',
          thinking: ['type' => 'enabled', 'budget_tokens' => 8000],
      );

      echo "Third response usage: " . json_encode($response3->usage) . "\n";
      ```

      ```ruby Ruby
      require "net/http"
      require "uri"

      def fetch_article_content(url)
        uri = URI.parse(url)
        response = Net::HTTP.get_response(uri)
        text = response.body

        lines = text.split("\n").map(&:strip)
        lines.reject(&:empty?).join("\n")
      end

      client = Anthropic::Client.new

      book_url = "https://www.gutenberg.org/cache/epub/1342/pg1342.txt"
      book_content = fetch_article_content(book_url)
      large_text = book_content[0...10000]

      puts "First request - establishing cache"
      response1 = client.messages.create(
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: {
          type: "enabled",
          budget_tokens: 4000
        },
        messages: [{
          role: "user",
          content: [
            {
              type: "text",
              text: large_text,
              cache_control: { type: "ephemeral" }
            },
            {
              type: "text",
              text: "Analyze the tone of this passage."
            }
          ]
        }]
      )

      puts "First response usage: #{response1.usage}"

      puts "\nSecond request - same thinking parameters (cache hit expected)"
      response2 = client.messages.create(
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: {
          type: "enabled",
          budget_tokens: 4000
        },
        messages: [
          {
            role: "user",
            content: [
              {
                type: "text",
                text: large_text,
                cache_control: { type: "ephemeral" }
              },
              {
                type: "text",
                text: "Analyze the tone of this passage."
              }
            ]
          },
          {
            role: "assistant",
            content: response1.content
          },
          {
            role: "user",
            content: "Analyze the characters in this passage."
          }
        ]
      )

      puts "Second response usage: #{response2.usage}"

      puts "\nThird request - different thinking budget (cache miss expected)"
      response3 = client.messages.create(
        model: "claude-sonnet-4-6",
        max_tokens: 20000,
        thinking: {
          type: "enabled",
          budget_tokens: 8000
        },
        messages: [
          {
            role: "user",
            content: [
              {
                type: "text",
                text: large_text,
                cache_control: { type: "ephemeral" }
              },
              {
                type: "text",
                text: "Analyze the tone of this passage."
              }
            ]
          },
          {
            role: "assistant",
            content: response1.content
          },
          {
            role: "user",
            content: "Analyze the characters in this passage."
          },
          {
            role: "assistant",
            content: response2.content
          },
          {
            role: "user",
            content: "Analyze the setting in this passage."
          }
        ]
      )

      puts "Third response usage: #{response3.usage}"
      ```
    </CodeGroup>

    Here is the output of the script (you may see slightly different numbers)

    ```text Output wrap
    First request - establishing cache
    First response usage: { cache_creation_input_tokens: 1370, cache_read_input_tokens: 0, input_tokens: 17, output_tokens: 700 }

    Second request - same thinking parameters (cache hit expected)

    Second response usage: { cache_creation_input_tokens: 0, cache_read_input_tokens: 1370, input_tokens: 303, output_tokens: 874 }

    Third request - different thinking budget (cache miss expected)
    Third response usage: { cache_creation_input_tokens: 1370, cache_read_input_tokens: 0, input_tokens: 747, output_tokens: 619 }
    ```

    This example demonstrates that when caching is set up in the messages array, changing the thinking parameters (budget\_tokens increased from 4000 to 8000) **invalidates the cache**. The third request shows no cache hit with `cache_creation_input_tokens=1370` and `cache_read_input_tokens=0`, proving that message-based caching is invalidated when thinking parameters change.
  </Accordion>
</AccordionGroup>

## Max tokens and context window size with extended thinking

`max_tokens` (which includes your thinking budget when thinking is enabled) is enforced as a strict limit. On Claude 4.5 models and newer, if input tokens plus `max_tokens` exceeds the context window size, the API accepts the request. If generation then reaches the context window limit, it stops with `stop_reason: "model_context_window_exceeded"`. On earlier models, the API returns a validation error instead. See [Handling stop reasons](/docs/en/build-with-claude/handling-stop-reasons).

<Note>
  You can read through the [guide on context windows](/docs/en/build-with-claude/context-windows) for a more thorough deep dive.
</Note>

### The context window with extended thinking

When calculating context window usage with thinking enabled, there are some considerations to be aware of:

* On Opus 4.5+ and Sonnet 4.6+, thinking blocks from previous turns are kept and count towards your context window; on earlier Opus/Sonnet models and all Haiku models, they are stripped and not counted
* Current turn thinking counts towards your `max_tokens` limit for that turn

The diagram below demonstrates the specialized token management when extended thinking is enabled:

![Context window diagram with extended thinking](/docs/images/context-window-thinking.svg)

The effective context window is calculated as:

```text wrap
context window =
  (current input tokens - previous thinking tokens) +
  (thinking tokens + encrypted thinking tokens + text output tokens)
```

Use the [token counting API](/docs/en/build-with-claude/token-counting) to get accurate token counts for your specific use case, especially when working with multi-turn conversations that include thinking.

### The context window with extended thinking and tool use

When using extended thinking with tool use, thinking blocks must be explicitly preserved and returned with the tool results.

The effective context window calculation for extended thinking with tool use becomes:

```text wrap
context window =
  (current input tokens + previous thinking tokens + tool use tokens) +
  (thinking tokens + encrypted thinking tokens + text output tokens)
```

The diagram below illustrates token management for extended thinking with tool use:

![Context window diagram with extended thinking and tool use](/docs/images/context-window-thinking-tools.svg)

### Managing tokens with extended thinking

Given the context window and `max_tokens` behavior with extended thinking, you may need to:

* More actively monitor and manage your token usage
* Adjust `max_tokens` values as your prompt length changes
* Potentially use the [token counting endpoints](/docs/en/build-with-claude/token-counting) more frequently
* Be aware that previous thinking blocks don't accumulate in your context window

## Thinking encryption

Full thinking content is encrypted and returned in the `signature` field. This field is used to verify that thinking blocks were generated by Claude when passed back to the API.

<Note>
  It is only strictly necessary to send back thinking blocks when using [tools with extended thinking](/docs/en/build-with-claude/extended-thinking#extended-thinking-with-tool-use). Otherwise you can omit thinking blocks from previous turns. If you pass them back, whether the API keeps or strips them depends on the model: Opus 4.5+ and Sonnet 4.6+ keep them in context by default; earlier Opus/Sonnet models and all Haiku models strip them. See [context editing](/docs/en/build-with-claude/context-editing) to configure this.

  If sending back thinking blocks, pass everything back as you received it for consistency and to avoid potential issues.
</Note>

Here are some important considerations on thinking encryption:

* When [streaming responses](/docs/en/build-with-claude/extended-thinking#streaming-thinking), the signature is added via a `signature_delta` inside a `content_block_delta` event just before the `content_block_stop` event.
* `signature` values are significantly longer in Claude 4 models than in previous models.
* The `signature` field is an opaque field and should not be interpreted or parsed.
* `signature` values are compatible across platforms (Claude APIs, [Amazon Bedrock](/docs/en/build-with-claude/claude-in-amazon-bedrock), and [Google Cloud](/docs/en/build-with-claude/claude-on-vertex-ai)). Values generated on one platform will be compatible with another.

## Redacted thinking blocks

In addition to regular `thinking` blocks, the API may return `redacted_thinking` blocks. A `redacted_thinking` block contains encrypted thinking content in a `data` field, with no readable summary:

```json
{
  "type": "redacted_thinking",
  "data": "..."
}
```

The `data` field is opaque and encrypted. Like the `signature` field on regular thinking blocks, you should pass `redacted_thinking` blocks back to the API unchanged when continuing a multi-turn conversation with [tools](/docs/en/build-with-claude/extended-thinking#extended-thinking-with-tool-use).

<Tip>
  If your code filters content blocks by type (for example, `block.type == "thinking"`) when round-tripping responses with tool use, also include `redacted_thinking` blocks. Filtering on `block.type == "thinking"` alone silently drops `redacted_thinking` blocks and breaks the multi-turn protocol described above.
</Tip>

<Note>
  `redacted_thinking` blocks are a distinct content block type returned by the API when portions of thinking are safety-redacted. This is separate from the [`display: "omitted"`](#controlling-thinking-display) option, which returns regular `thinking` blocks with an empty `thinking` field.
</Note>

## Differences in thinking across model versions

The Messages API handles thinking differently across Claude model versions. The following table gives a condensed comparison:

| Model                                                    | `budget_tokens` | Thinking output     | Interleaved thinking      | Block preservation                                                    |
| -------------------------------------------------------- | --------------- | ------------------- | ------------------------- | --------------------------------------------------------------------- |
| Claude Fable 5 Claude Mythos 5                           | Not supported   | Omitted by default1 | Automatic2                | See [Adaptive thinking](/docs/en/build-with-claude/adaptive-thinking) |
| [Claude Mythos Preview](https://anthropic.com/glasswing) | Supported       | Omitted by default1 | Automatic2                | Preserved3                                                            |
| Claude Opus 4.8                                          | Not supported   | Omitted by default1 | Automatic2                | Preserved                                                             |
| Claude Opus 4.7                                          | Not supported   | Omitted by default1 | Automatic2                | Preserved                                                             |
| Claude Sonnet 5                                          | Not supported   | Omitted by default1 | Automatic2                | Preserved                                                             |
| Claude Opus 4.6                                          | Deprecated      | Summarized          | Automatic2                | Preserved                                                             |
| Claude Sonnet 4.6                                        | Deprecated      | Summarized          | Automatic, or beta header | Preserved                                                             |
| Claude Opus 4.5                                          | Supported       | Summarized          | Beta header               | Preserved                                                             |
| Claude Haiku 4.5                                         | Supported       | Summarized          | Not supported             | Last turn only                                                        |
| Earlier Claude 4 models                                  | Supported       | Summarized          | Beta header               | Last turn only                                                        |

*1 Set `display: "summarized"` to receive summarized thinking. On Claude Fable 5, Claude Mythos 5, and Claude Mythos Preview, raw thinking tokens are never returned.*\
*2 With [adaptive thinking](/docs/en/build-with-claude/adaptive-thinking). The `interleaved-thinking-2025-05-14` beta header is not needed on these models and is safely ignored if included.*\
*3 Blocks are stripped when continuing the conversation on a model that does not support the Mythos thinking format.*

### Thinking block preservation by model

Whether thinking blocks from previous assistant turns are preserved in context by default depends on the model class. **Opus**: Claude Opus 4.5 and later Opus models keep all prior thinking blocks; Claude Opus 4.1 (deprecated) and earlier Opus models keep only the last assistant turn's thinking. **Sonnet**: Claude Sonnet 4.6 and later Sonnet models keep all; Claude Sonnet 4.5 and earlier Sonnet models keep only the last turn. **Haiku**: all Haiku models through Claude Haiku 4.5 keep only the last turn. [Claude Mythos Preview](https://anthropic.com/glasswing) also keeps all prior thinking blocks.

**Benefits of thinking block preservation:**

* **Cache optimization**: When using tool use, preserved thinking blocks enable cache hits as they are passed back with tool results and cached incrementally across the assistant turn, resulting in token savings in multi-step workflows
* **No intelligence impact**: Preserving thinking blocks has no negative effect on model performance

**Important considerations:**

* **Context usage**: Long conversations will consume more context space since thinking blocks are retained in context
* **Automatic behavior**: This is the default for each model as listed above. No code changes or beta headers are required
* **Backward compatibility**: To leverage this feature, continue passing complete, unmodified thinking blocks back to the API as you would for tool use

<Note>
  For earlier models (Claude Sonnet 4.5, Opus 4.1 (deprecated), etc.), thinking blocks from previous turns continue to be removed from context. The existing behavior described in the [Extended thinking with prompt caching](#extended-thinking-with-prompt-caching) section applies to those models.
</Note>

## Pricing

For complete pricing information including base rates, cache writes, cache hits, and output tokens, see the [pricing page](/docs/en/about-claude/pricing).

The thinking process incurs charges for:

* Tokens used during thinking (output tokens)
* Thinking blocks from prior assistant turns kept in context: only the last turn on earlier Opus/Sonnet models and all Haiku models; all turns by default on Opus 4.5+ and Sonnet 4.6+ (input tokens)
* Standard text output tokens

<Note>
  When extended thinking is enabled, a specialized system prompt is automatically included to support this feature.
</Note>

When using summarized thinking:

* **Input tokens:** Tokens in your original request (excludes thinking tokens from previous turns)
* **Output tokens (billed):** The original thinking tokens that Claude generated internally
* **Output tokens (visible):** The summarized thinking tokens you see in the response
* **No charge:** Tokens used to generate the summary

When using `display: "omitted"`:

* **Input tokens:** Tokens in your original request (same as summarized)
* **Output tokens (billed):** The original thinking tokens that Claude generated internally (same as summarized)
* **Output tokens (visible):** Zero thinking tokens (the `thinking` field is empty)

<Warning>
  The billed output token count will **not** match the visible token count in the response. You are billed for the full thinking process, not the thinking content visible in the response.
</Warning>

To see how many billed output tokens were spent on internal reasoning, read `usage.output_tokens_details.thinking_tokens` in the response. This value reflects the raw reasoning the model generated (not the summarized text returned in the body) and is always less than or equal to `output_tokens`. Subtract it from `output_tokens` to approximate the non-reasoning portion of the output.

```json
{
  "usage": {
    "input_tokens": 25,
    "output_tokens": 348,
    "output_tokens_details": {
      "thinking_tokens": 312
    }
  }
}
```

`output_tokens` remains the inclusive, authoritative total used for billing. `output_tokens_details` is a read-only breakdown for observability.

## Best practices and considerations for extended thinking

### Working with thinking budgets

* **Budget optimization:** The minimum budget is 1,024 tokens. Start at the minimum and increase the thinking budget incrementally to find the optimal range for your use case. Higher token counts enable more comprehensive reasoning but with diminishing returns depending on the task. Increasing the budget can improve response quality at the tradeoff of increased latency. For critical tasks, test different settings to find the optimal balance. Note that the thinking budget is a target rather than a strict limit. Actual token usage may vary based on the task.
* **Starting points:** Start with larger thinking budgets (16k+ tokens) for complex tasks and adjust based on your needs.
* **Large budgets:** For thinking budgets above 32k, use [batch processing](/docs/en/build-with-claude/batch-processing) to avoid networking issues. Requests pushing the model to think above 32k tokens causes long running requests that might run up against system timeouts and open connection limits.
* **Token usage tracking:** Monitor thinking token usage to optimize costs and performance. The `usage.output_tokens_details.thinking_tokens` field in the response reports how many of the billed output tokens were internal reasoning. When streaming, this breakdown appears only on the final `message_delta` event.

### Performance considerations

* **Response times:** Be prepared for longer response times due to additional processing. Generating thinking blocks increases overall response time.
* **Streaming requirements:** The SDKs require streaming when `max_tokens` is greater than 21,333 to avoid HTTP timeouts on long-running requests. This is a client-side validation, not an API restriction. If you don't need to process events incrementally, use `.stream()` with `.get_final_message()` (Python) or `.finalMessage()` (TypeScript) to get the complete `Message` object without handling individual events. See [Streaming Messages](/docs/en/build-with-claude/streaming#get-the-final-message-without-handling-events) for details. When streaming, be prepared to handle both thinking and text content blocks as they arrive.
* **Omitting thinking for latency:** If your application doesn't display thinking content, set `display: "omitted"` on the thinking configuration to reduce time-to-first-text-token. See [Controlling thinking display](#controlling-thinking-display).

### Feature compatibility

* Thinking isn't compatible with `temperature` or `top_k` modifications as well as [forced tool use](/docs/en/agents-and-tools/tool-use/define-tools#forcing-tool-use).
* When thinking is enabled, you can set `top_p` to values between 1 and 0.95.
* You can't pre-fill responses when thinking is enabled.
* Changes to the thinking budget invalidate cached prompt prefixes that include messages. However, cached system prompts and tool definitions will continue to work when thinking parameters change.

### Usage guidelines

* **Task selection:** Use extended thinking for particularly complex tasks that benefit from step-by-step reasoning, like math, coding, and analysis.
* **Context handling:** You don't need to remove previous thinking blocks yourself. On Opus 4.5+ and Sonnet 4.6+, the Claude API keeps thinking blocks from previous turns by default; on earlier Opus/Sonnet models and all Haiku models, it automatically ignores them and they aren't included when calculating context usage.
* **Prompt engineering:** Review the [extended thinking prompting tips](/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#leverage-thinking-and-interleaved-thinking-capabilities) if you want to maximize Claude's thinking capabilities.

## Next steps

<CardGroup cols={3}>
  <Card title="Adaptive thinking" icon="brain" href="/docs/en/build-with-claude/adaptive-thinking">
    Let Claude decide when and how much to use extended thinking.
  </Card>

  <Card title="Try the extended thinking cookbook" icon="book" href="https://platform.claude.com/cookbook/extended-thinking-extended-thinking">
    Explore practical examples of thinking in the cookbook.
  </Card>

  <Card title="Extended thinking prompting tips" icon="code" href="/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#leverage-thinking-and-interleaved-thinking-capabilities">
    Learn prompt engineering best practices for extended thinking.
  </Card>
</CardGroup>
