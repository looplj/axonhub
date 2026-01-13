package objects

type PromptActionType string

const (
	// PromptActionTypePrepend is the action to prepend the prompt before the request messages.
	PromptActionTypePrepend PromptActionType = "prepend"

	// PromptActionTypeAppend is the action to append the prompt after the request messages.
	PromptActionTypeAppend PromptActionType = "append"
)

// PromptAction is the action to perform when the prompt is activated.
type PromptAction struct {
	// Type is the type of prompt action.
	// It is continue to add more action types in the future.
	Type PromptActionType `json:"type"`
}

type PromptActivationConditionType string

const (
	// PromptActivationConditionTypeModelID is the condition to activate the prompt for the specified model ID.
	PromptActivationConditionTypeModelID PromptActivationConditionType = "model_id"

	// PromptActivationConditionTypeModelPattern is the condition to activate the prompt for the models that match the pattern.
	PromptActivationConditionTypeModelPattern PromptActivationConditionType = "model_pattern"
)

// PromptActivationCondition is the condition to activate the prompt.
type PromptActivationCondition struct {
	// Type is the type of prompt activation condition.
	// It is continue to add more condition types in the future.
	Type PromptActivationConditionType `json:"type"`

	// ModelID is the ID of the model to activate the prompt.
	ModelID *string `json:"model_id,omitempty"`

	// ModelPattern is the pattern of the model to activate the prompt.
	// The pattern is a regular expression.
	ModelPattern *string `json:"model_pattern,omitempty"`
}

// PromptActivationConditionComposite is the composite condition to activate the prompt.
type PromptActivationConditionComposite struct {
	// Conditions is the conditions to activate the prompt.
	// At least one condition must be met to activate the prompt.
	Conditions []PromptActivationCondition `json:"conditions,omitempty"`
}

type PromptSettings struct {
	// Action is the action to perform when the prompt is activated.
	Action PromptAction `json:"action"`

	// Conditions of the prompts must to be met to activate the prompt.
	// All conditions must be met to activate the prompt.
	Conditions []PromptActivationConditionComposite `json:"conditions,omitempty"`
}
