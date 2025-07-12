package log

import (
	"encoding/json"

	"go.uber.org/zap/zapcore"
)

func EncodeAny(encoder zapcore.ObjectEncoder, key string, obj any) error {
	switch raw := obj.(type) {
	case zapcore.ObjectMarshaler:
		return encoder.AddObject(key, raw)
	case json.Marshaler:
		return encoder.AddReflected(key, raw)
	case error:
		// must behind json.Marshaler and ObjectMarshaler
		encoder.AddString(key, raw.Error())
		return nil
	default:
		return encoder.AddReflected(key, raw)
	}
}
