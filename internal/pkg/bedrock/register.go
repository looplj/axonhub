package bedrock

import (
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// init registers the AWS EventStream decoder.
func init() {
	httpclient.RegisterDecoder("application/vnd.amazon.eventstream", NewAWSEventStreamDecoder)
}
