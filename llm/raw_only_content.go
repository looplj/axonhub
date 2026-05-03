package llm

const TransformerMetadataRawOnlyContentPresent = "raw_only_content_present"

func MarkRawOnlyResponseContent(resp *Response) {
	if resp == nil {
		return
	}

	if resp.TransformerMetadata == nil {
		resp.TransformerMetadata = make(map[string]any)
	}

	resp.TransformerMetadata[TransformerMetadataRawOnlyContentPresent] = true
}

func HasRawOnlyResponseContent(resp *Response) bool {
	if resp == nil || resp.TransformerMetadata == nil {
		return false
	}

	present, ok := resp.TransformerMetadata[TransformerMetadataRawOnlyContentPresent].(bool)
	return ok && present
}
