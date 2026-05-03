package responses

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func ignoreResponsesRawCaptureOptions() []cmp.Option {
	return []cmp.Option{
		cmpopts.IgnoreFields(Request{}, "Raw", "Extra", "MetadataRaw"),
		cmpopts.IgnoreFields(Response{}, "Raw", "Extra", "MetadataRaw", "OutputRaw"),
		cmpopts.IgnoreFields(Input{}, "Raw"),
		cmpopts.IgnoreFields(Item{}, "Raw", "Extra"),
		cmpopts.IgnoreFields(StreamEvent{}, "Raw", "Extra"),
		cmpopts.IgnoreFields(Tool{}, "Raw", "Extra", "ParametersRaw"),
		cmpopts.IgnoreFields(ToolChoice{}, "Raw", "Extra"),
	}
}
