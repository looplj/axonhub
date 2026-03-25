package agent

func newTextMessage(role Role, text string) Message {
	return Message{
		Role:    role,
		Content: &Content{Text: &text},
	}
}

func cloneMessages(in []Message) []Message {
	if in == nil {
		return nil
	}
	out := make([]Message, len(in))
	for i, msg := range in {
		out[i] = cloneMessage(msg)
	}
	return out
}

func cloneMessage(in Message) Message {
	out := Message{
		Role:       in.Role,
		RoundIndex: in.RoundIndex,
	}

	if in.Content != nil {
		c := &Content{}

		if in.Content.Text != nil {
			s := *in.Content.Text
			c.Text = &s
		}

		if len(in.Content.Parts) > 0 {
			c.Parts = make([]ContentPart, len(in.Content.Parts))
			copy(c.Parts, in.Content.Parts)
		}

		out.Content = c
	}

	if in.ToolUseID != nil {
		s := *in.ToolUseID
		out.ToolUseID = &s
	}

	if in.IsError != nil {
		b := *in.IsError
		out.IsError = &b
	}

	if in.ToolCall != nil {
		tu := *in.ToolCall
		out.ToolCall = &tu
	}

	return out
}

func countUniqueRounds(messages []Message) int {
	seen := make(map[int]struct{})

	for _, msg := range messages {
		if msg.RoundIndex != 0 {
			seen[msg.RoundIndex] = struct{}{}
		}
	}

	return len(seen)
}

func findCutIndexForRounds(messages []Message, keepRounds int) int {
	if keepRounds <= 0 || len(messages) == 0 {
		return 0
	}

	roundSet := make(map[int]struct{})

	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].RoundIndex != 0 {
			roundSet[messages[i].RoundIndex] = struct{}{}
		}

		if len(roundSet) > keepRounds {
			return i + 1
		}
	}

	return 0
}
