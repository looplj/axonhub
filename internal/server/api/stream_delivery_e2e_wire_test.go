package api

import "fmt"

const streamDeliveryBlockingResponseMarker = "__stream_delivery_blocking_response__"

func streamDeliveryBlockingResponsePrefix() string {
	return "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-blocked\",\"status\":\"in_progress\",\"output\":[]}}\n\nevent: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg-blocked\",\"output_index\":0,\"content_index\":0,\"delta\":\"working\"}\n\n"
}

func streamDeliveryChat(input, output int, done bool) string {
	result := fmt.Sprintf("data: {\"id\":\"chat\",\"model\":\"gpt-test\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: {\"id\":\"chat\",\"model\":\"gpt-test\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: {\"id\":\"chat\",\"model\":\"gpt-test\",\"choices\":[],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d,\"total_tokens\":%d}}\n\n", input, output, input+output)
	if done {
		result += "data: [DONE]\n\n"
	}
	return result
}

func streamDeliveryChat2616PrematureTerminalVariant() string {
	return "data: {\"id\":\"chat\",\"choices\":[{\"delta\":{},\"finish_reason\":null}],\"usage\":{\"prompt_tokens\":0,\"completion_tokens\":0,\"total_tokens\":0}}\n\ndata: {\"id\":\"chat\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":0,\"completion_tokens\":0,\"total_tokens\":0}}\n\ndata: [DONE]\n\n"
}

func streamDeliveryResponsesMissing() string {
	return "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp\",\"status\":\"in_progress\",\"output\":[]}}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp\",\"status\":\"completed\",\"output\":[]}}\n\n"
}

func streamDeliveryAnthropic(input, output int) string {
	return fmt.Sprintf("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"claude\",\"usage\":{\"input_tokens\":%d,\"output_tokens\":0}}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":%d,\"output_tokens\":%d}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", input, input, output)
}
