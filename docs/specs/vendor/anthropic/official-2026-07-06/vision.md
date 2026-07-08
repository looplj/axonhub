### Cookie settings

We use cookies to deliver and improve our services, analyze site usage, and if you agree, to customize or personalize your experience and market our services to you. You can read our Cookie Policy [here](https://www.anthropic.com/legal/cookies).

[Claude Platform Docs](/docs/en/home)

Overview

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

Messages/Images and vision

# Vision

Claude's vision capabilities allow it to understand and analyze images, opening up exciting possibilities for multimodal interaction.

This guide describes how to send images to Claude, the limits and costs that apply, and where to find guidance for [coordinate-based workflows](/docs/en/build-with-claude/vision-coordinates).

---

## Send images to Claude

Use Claude's vision capabilities through:

* [claude.ai](https://claude.ai/). Upload an image like you would a file, or drag and drop an image directly into the chat window.
* The [Anthropic Workbench](/workbench). A button to add images appears at the top right of every User message block.
* API request. See the following examples.

On the API, provide images to Claude as `image` content blocks using one of three source types:

1. A base64-encoded image embedded in the request body
2. A URL reference to an image hosted online
3. A `file_id` returned by the [Files API](/docs/en/build-with-claude/files) (upload once, reference many times)



On Amazon Bedrock and Google Cloud, only base64-encoded sources are currently available.



Just as [placing long documents before your query](/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#long-context-prompting) improves results in text prompts, Claude works best when images come before text. Images placed after text or interpolated with text still perform well, but if your use case allows it, prefer an image-then-text structure.

### Base64-encoded image example

```
image1_data = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC" image1_data = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC"image1_media_type = "image/png" image1_media_type = "image/png" client = anthropic.Anthropic() client = anthropic.Anthropic()message = client.messages.create(message = client.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, messages=[ messages =[ { { "role": "user",  "role": "user", "content": [ "content": [ { { "type": "image",  "type": "image", "source": { "source": { "type": "base64",  "type": "base64", "media_type": image1_media_type,  "media_type": image1_media_type, "data": image1_data,  "data": image1_data, }, }, }, }, {"type": "text", "text": "Describe this image."}, {"type": "text", "text": "Describe this image."}, ], ], } } ], ],))print(message) print(message)
```

### URL-based image example

```
client = anthropic.Anthropic() client = anthropic.Anthropic()message = client.messages.create(message = client.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, messages=[ messages =[ { { "role": "user",  "role": "user", "content": [ "content": [ { { "type": "image",  "type": "image", "source": { "source": { "type": "url",  "type": "url", "url": "https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg",  "url": "https://upload.wikimedia.org/wikipedia/commons/a/a7/Camponotus_flavomarginatus_ant.jpg", }, }, }, }, {"type": "text", "text": "Describe this image."}, {"type": "text", "text": "Describe this image."}, ], ], } } ], ],))print(message) print(message)
```

### Files API image example

For images you'll use repeatedly or when you want to avoid encoding overhead, use the [Files API](/docs/en/build-with-claude/files). Upload the image once, then reference the returned `file_id` in subsequent messages instead of resending base64 data.



In multi-turn conversations and agentic workflows, each request resends the full conversation history. If images are base64-encoded, the full image bytes are included in the payload on every turn, which can significantly increase request size and latency as the conversation grows. Uploading images to the Files API and referencing them by `file_id` keeps request payloads small regardless of how many images accumulate in the conversation history.

```
client = anthropic.Anthropic() client = anthropic.Anthropic() # Upload the image file # Upload the image filewith open("image.jpg", "rb") as f: with  open("image.jpg", "rb") as f: file_upload = client.beta.files.upload(file=("image.jpg", f, "image/jpeg"))  file_upload = client.beta.files.upload(file =("image.jpg", f, "image/jpeg")) # Use the uploaded file in a message # Use the uploaded file in a messagemessage = client.beta.messages.create(message = client.beta.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, betas=["files-api-2025-04-14"],  betas =["files-api-2025-04-14"], messages=[ messages =[ { { "role": "user",  "role": "user", "content": [ "content": [ { { "type": "image",  "type": "image", "source": {"type": "file", "file_id": file_upload.id},  "source": {"type": "file", "file_id": file_upload.id}, }, }, {"type": "text", "text": "Describe this image."}, {"type": "text", "text": "Describe this image."}, ], ], } } ], ],)) print(message.content) print(message.content)
```

See [Messages API examples](/docs/en/api/messages/create) for more example code and parameter details.

### Multiple images

You can include multiple images in a single request, and Claude analyzes them jointly. This is useful for comparing images, asking about differences, or working with a sequence such as pages of a document. When sending several images, introduce each one with a short text label (`Image 1:`, `Image 2:`, and so on) so you can refer to them by name in your prompt and in follow-up turns.

Python

```
client = anthropic.Anthropic() client = anthropic.Anthropic()message = client.messages.create(message = client.messages.create( model="claude-opus-4-8",  model ="claude-opus-4-8", max_tokens=1024,  max_tokens = 1024, messages=[ messages =[ { { "role": "user",  "role": "user", "content": [ "content": [ {"type": "text", "text": "Image 1:"}, {"type": "text", "text": "Image 1:"}, { { "type": "image",  "type": "image", "source": { "source": { "type": "base64",  "type": "base64", "media_type": "image/png",  "media_type": "image/png", "data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC",  "data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC", }, }, }, }, {"type": "text", "text": "Image 2:"}, {"type": "text", "text": "Image 2:"}, { { "type": "image",  "type": "image", "source": { "source": { "type": "base64",  "type": "base64", "media_type": "image/png",  "media_type": "image/png", "data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGNgYPgPAAEDAQAIicLsAAAAAElFTkSuQmCC",  "data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGNgYPgPAAEDAQAIicLsAAAAAElFTkSuQmCC", }, }, }, }, {"type": "text", "text": "How are these images different?"}, {"type": "text", "text": "How are these images different?"}, ], ], } } ], ],))print(message) print(message)
```

In a multi-turn conversation, add new images in later `user` turns the same way. Claude has access to every image from earlier turns, so follow-up questions such as "Are these similar to the first two?" work without including the earlier images again in the new turn's content.

---

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



Although the API supports up to 600 images per request, [request size limits](/docs/en/api/overview#request-size-limits) (32 MB for standard endpoints; lower on some partner-operated platforms, for example, Amazon Bedrock and Google Cloud) can be reached first. For many images, consider uploading with the [Files API](#files-api-image-example) and referencing by `file_id` to keep request payloads small.

Even when using the Files API, requests with many large images can fail before reaching the 600-image count. Reduce image dimensions or file sizes (for example, by downsampling) before uploading (see [Resolution and token cost](#evaluate-image-size)).

### Supported formats

Claude supports JPEG, PNG, GIF, and WebP images (`image/jpeg`, `image/png`, `image/gif`, `image/webp`). Animations are unsupported, and only the first frame is used.

### Resolution and token cost

Claude views images in patches instead of pixels. Each patch is a 28×28-pixel block of the image, referred to as a visual token. An image, therefore, costs `⌈width / 28⌉ × ⌈height / 28⌉` visual tokens.

Each model has a maximum native image resolution, expressed as a long-edge limit and a visual-token limit. Images larger than either limit are downscaled before processing; see [How Claude resizes and pads images](/docs/en/build-with-claude/vision-coordinates#how-claude-resizes-and-pads-images) for the exact rule.

| Resolution tier | Models | Max long edge | Max visual tokens |
| --- | --- | --- | --- |
| High-resolution | Claude Fable 5, Claude Mythos 5, Claude Opus 4.8, Claude Opus 4.7, Claude Sonnet 5 | 2576 px | 4784 |
| Standard | All other models | 1568 px | 1568 |

High-resolution support is automatic on the listed models and requires no beta header or client-side opt-in.

The following table shows the visual-token cost for several image sizes on each tier:

| Image size | Standard-tier tokens | High-resolution-tier tokens |
| --- | --- | --- |
| 200x200 px (0.04 megapixels) | 64 | 64 |
| 1000x1000 px (1 megapixel) | 1296 | 1296 |
| 1092x1092 px (1.19 megapixels) | 1521 | 1521 |
| 1920x1080 px (2.07 megapixels) | 1560 | 2691 |
| 2000x1500 px (3 megapixels) | 1564 | 3888 |
| 3840x2160 px (8.29 megapixels) | 1560 | 4784 |

To estimate cost, multiply the token count by the [per-token price of the model](https://claude.com/pricing) you're using. For example, at Claude Haiku 4.5's $1 per million input tokens (standard tier), the 1000×1000 image costs about $1.30 per thousand images. At Claude Opus 4.8's $5 per million (high-resolution tier), the same image costs about $6.48 per thousand and the 4K image about $23.92 per thousand.

High-resolution images can use up to roughly three times more visual tokens than the same image on a standard-tier model. If you don't need the additional fidelity that high resolution provides for computer use, screenshot understanding, and dense documents, downsample images before sending to control token costs. To minimize latency and to simplify [coordinate-based workflows](/docs/en/build-with-claude/vision-coordinates), prefer resizing images before uploading them.

### Image quality guidance

When providing images to Claude, keep the following in mind for best results:

* **Image clarity:** Ensure images are clear and not too blurry or pixelated.
* **Text:** If the image contains important text, make sure it's legible and not too small. Avoid cropping out key visual context solely to enlarge the text.
* **Resizing:** Take into account that your image might be resized if it is too large (see [Resolution and token cost](#evaluate-image-size)); this might, for example, make text less legible. Consider pre-resizing your images, cropping them, or both.
* **Image compression:** Compressing images before sending them, using a lossy format such as JPEG or WebP (lossy mode), can reduce latency by reducing the size of requests. However, this can introduce artifacts that are detrimental to model performance, especially when multiple compression passes are applied. For example, heavy JPEG compression can make text difficult to read. Confirm your compression settings are appropriate for the task by inspecting the actual images sent to the API.

---

## Coordinates and bounding boxes

For bounding boxes, points, and pixel coordinates, see [Coordinates and bounding boxes](/docs/en/build-with-claude/vision-coordinates). Claude returns absolute pixel coordinates relative to the image it sees after resizing; that guide covers how Claude resizes and pads images and how to pre-resize or rescale so coordinates line up with your original image.

---

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

---

## FAQ

---

## Next steps

[Multimodal cookbook



Get tips and best-practice techniques for tasks such as interpreting charts and extracting content from forms.](https://platform.claude.com/cookbook/multimodal-getting-started-with-vision)[API reference

See the Messages API documentation, including example API calls involving images.](/docs/en/api/messages/create)

Was this page helpful?
