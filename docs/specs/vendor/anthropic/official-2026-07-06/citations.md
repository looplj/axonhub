### Cookie settings

We use cookies to deliver and improve our services, analyze site usage, and if you agree, to customize or personalize your experience and market our services to you. You can read our Cookie Policy [here](https://www.anthropic.com/legal/cookies).

### Solutions

### Partners

### Learn

### Company

### Learn

### Help and security

### Terms and policies

# Citations

This feature is eligible for [Zero Data Retention (ZDR)](/docs/en/build-with-claude/api-and-data-retention). When your organization has a ZDR arrangement, data sent through this feature is not stored after the API response is returned.

Claude can provide detailed citations when answering questions about documents, helping you track and verify the sources behind each response.

All [active models](/docs/en/about-claude/models/overview) support citations, with the exception of Claude Haiku 3.

Share your feedback and suggestions about the citations feature using the [citations feedback form](https://forms.gle/9n9hSrKnKe3rpowH9).

The following example shows how to enable citations on a plain text document with the Messages API:

`client = anthropic.Anthropic()

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
print(response)`

**Comparison with prompt-based approaches**

Compared to prompting Claude to cite sources, the citations feature offers the following advantages:

`cited_text`
`cited_text`

##  How citations work

Integrate citations with Claude in these steps:

Provide document(s) and enable citations

`citations.enabled=true`

Documents get processed

Claude provides cited response

**Automatic chunking vs custom content**

By default, plain text and PDF documents are automatically chunked into sentences. If you need more control over citation granularity (for example, for bullet points or transcripts), use custom content documents instead. See [Document types](#document-types) for more details.

For example, if you want Claude to be able to cite specific sentences from your RAG chunks, you should put each RAG chunk into a plain text document. Otherwise, if you do not want any further chunking to be done, or if you want to customize any additional chunking, you can put RAG chunks into custom content document(s).

###  Citable vs non-citable content

`source`
`title`
`context`
`title`
`context`

###  Citation indices

`content`

###  Token costs

`cited_text`
`cited_text`

###  Feature compatibility

Citations work in conjunction with other API features including [prompt caching](/docs/en/build-with-claude/prompt-caching), [token counting](/docs/en/build-with-claude/token-counting), and [batch processing](/docs/en/build-with-claude/batch-processing).

**Citations and structured outputs are incompatible**

Citations cannot be used together with [structured outputs](/docs/en/build-with-claude/structured-outputs). If you enable citations on any user-provided document (`document` blocks or `search_result` blocks) and also include the `output_config.format` parameter (or the deprecated `output_format` parameter), the API returns a 400 error.

`document`
`search_result`
`output_config.format`
`output_format`

This is because citations require interleaving citation blocks with text output, which is incompatible with the strict JSON schema constraints of structured outputs.

####  Using prompt caching with citations

Citations and prompt caching can be used together effectively.

The citation blocks generated in responses cannot be cached directly, but the source documents they reference can be cached. To optimize performance, apply `cache_control` to your top-level document content blocks.

`cache_control`
`client = anthropic.Anthropic()

# Long document content (for example, technical documentation)
long_document = (
 "This is a very long document with thousands of words..." + " ... " * 1000
) # Minimum cacheable length

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
 }, # Cache the document content
 },
 {
 "type": "text",
 "text": "What does this document say about API features?",
 },
 ],
 }
 ],
)
print(response)`

In this example:

`cache_control`

##  Document types

###  Choosing a document type

Three document types are supported for citations. Documents can be provided directly in the message (base64, text, or URL) or uploaded through the [Files API](/docs/en/build-with-claude/files) and referenced by `file_id`:

`file_id`

| Type | Best for | Chunking | Citation format |
| --- | --- | --- | --- |
| Plain text | Simple text documents, prose | Sentence | Character indices (0-indexed) |
| PDF | PDF files with text content | Sentence | Page numbers (1-indexed) |
| Custom content | Lists, transcripts, special formatting, more granular citations | No additional chunking | Block indices (0-indexed) |

.csv, .xlsx, .docx, .md, and .txt files are not supported as document blocks. Convert these to plain text and include directly in message content. See [Working with other file formats](/docs/en/build-with-claude/files#working-with-other-file-formats).

###  Plain text documents

Plain text documents are automatically chunked into sentences. You can provide them inline or by reference with their `file_id`:

`file_id`

### Example plain text citation

###  PDF documents

PDF documents can be provided as base64-encoded data, a URL, or by `file_id`. PDF text is extracted and chunked into sentences. As image citations are not yet supported, PDFs that are scans of documents and do not contain extractable text will not be citable.

`file_id`

### Example PDF citation

###  Custom content documents

Custom content documents give you control over citation granularity. No additional chunking is done and chunks are provided to the model according to the content blocks provided.

`client = anthropic.Anthropic()

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
print(response)`

### Example citation

##  Response structure

When citations are enabled, responses include multiple text blocks with citations:

`{
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
}`

###  Streaming support

For streaming responses, citations arrive as a `citations_delta` delta type inside `content_block_delta` events. Each delta contains a single citation to add to the `citations` list on the current `text` content block.

`citations_delta`
`content_block_delta`
`citations`
`text`

### Example streaming events

##  Next steps

Handle the `citations_delta` delta type alongside text deltas to render cited responses as they stream.

`citations_delta`

Pass search results from your RAG pipeline as first-class content blocks with built-in citation support.

Learn how Claude extracts text from PDFs and how page-based citations map back to your source files.

Upload documents once and reference them by `file_id` across multiple citation requests.

`file_id`

Was this page helpful?
