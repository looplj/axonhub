package llm

import (
	"bytes"
	"encoding/json"
)

// SystemOneRequest represents a System One decision inference request.
type SystemOneRequest struct {
	// State is the context or entity to evaluate (can be string, map, or slice).
	State any `json:"state"`

	// Questions is the map of question identifiers to their schema definitions.
	Questions map[string]SystemOneQuestion `json:"questions"`
}

// SystemOneQuestion defines a single decision question schema.
type SystemOneQuestion struct {
	// Type specifies the primitive type: "choice", "noul", or "score".
	Type string `json:"type"`

	// Instructions guides the decision model on how to evaluate this question (string, object, or array).
	Instructions any `json:"instructions"`

	// Criteria holds optional evaluation criteria/rubric.
	Criteria any `json:"criteria,omitempty"`
}

// SystemOneResponse represents the unified result of a System One decision inference.
type SystemOneResponse struct {
	// Answers maps question identifiers to their evaluated results.
	Answers map[string]SystemOneAnswer `json:"answers"`
}

// SystemOneAnswer holds the evaluated answer for a specific question.
type SystemOneAnswer struct {
	Type          string                     `json:"type,omitempty"`
	Choice        string                     `json:"choice,omitempty"`
	Noul          *float64                   `json:"noul,omitempty"`
	Score         *float64                   `json:"score,omitempty"`
	Legend        any                        `json:"legend,omitempty"`
	Probabilities any                        `json:"probabilities,omitempty"`
	Confidence    *float64                   `json:"confidence,omitempty"`
	Extra         map[string]json.RawMessage `json:"-"`
}

func (a *SystemOneAnswer) UnmarshalJSON(data []byte) error {
	type Alias SystemOneAnswer
	var aux Alias
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&aux); err != nil {
		return err
	}
	*a = SystemOneAnswer(aux)

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	delete(rawMap, "type")
	delete(rawMap, "choice")
	delete(rawMap, "noul")
	delete(rawMap, "score")
	delete(rawMap, "legend")
	delete(rawMap, "probabilities")
	delete(rawMap, "confidence")

	if len(rawMap) > 0 {
		a.Extra = rawMap
	}
	return nil
}

func (a SystemOneAnswer) MarshalJSON() ([]byte, error) {
	type Alias SystemOneAnswer
	base, err := json.Marshal(Alias(a))
	if err != nil {
		return nil, err
	}
	if len(a.Extra) == 0 {
		return base, nil
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(base, &rawMap); err != nil {
		return nil, err
	}
	if rawMap == nil {
		rawMap = make(map[string]json.RawMessage)
	}
	for k, v := range a.Extra {
		rawMap[k] = v
	}
	return json.Marshal(rawMap)
}
