package responses

type PresenceOverlay struct {
	raw        map[string]any
	structured map[string]any
}

func NewPresenceOverlay(raw map[string]any) *PresenceOverlay {
	return &PresenceOverlay{
		raw:        cloneOverlayMap(raw),
		structured: make(map[string]any),
	}
}

func (o *PresenceOverlay) Set(field string, value any) {
	if o == nil || field == "" {
		return
	}

	if o.structured == nil {
		o.structured = make(map[string]any)
	}

	o.structured[field] = value
}

func (o *PresenceOverlay) Merge() map[string]any {
	if o == nil {
		return nil
	}

	merged := cloneOverlayMap(o.raw)
	for field, value := range o.structured {
		merged[field] = value
	}

	return merged
}

func cloneOverlayMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return make(map[string]any)
	}

	cloned := make(map[string]any, len(src))
	for key, value := range src {
		cloned[key] = value
	}

	return cloned
}
