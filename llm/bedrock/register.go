package bedrock

import (
	"github.com/looplj/axonhub/llm/httpclient"
)

// init registers the AWS EventStream decoder.
func init() {
	httpclient.RegisterDecoder("application/vnd.amazon.eventstream", NewAWSEventStreamDecoder)
}
