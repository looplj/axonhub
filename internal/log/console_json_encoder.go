package log

import (
	"encoding/json"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

type ConsoleJSONEncoder struct {
	zapcore.Encoder

	cfg zapcore.EncoderConfig
}

func NewConsoleJSONEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	if len(cfg.ConsoleSeparator) == 0 {
		// Use a default delimiter of '\t' for backwards compatibility
		cfg.ConsoleSeparator = "\t"
	}

	encoder := zapcore.NewConsoleEncoder(cfg)

	return &ConsoleJSONEncoder{
		Encoder: encoder,
		cfg:     cfg,
	}
}

func (c *ConsoleJSONEncoder) Clone() zapcore.Encoder {
	return &ConsoleJSONEncoder{Encoder: c.Encoder.Clone(), cfg: c.cfg}
}

func (enc *ConsoleJSONEncoder) EncodeEntry(
	ent zapcore.Entry,
	fields []zapcore.Field,
) (*buffer.Buffer, error) {
	line, err := enc.Encoder.EncodeEntry(ent, nil)
	if err != nil {
		return line, err
	}

	if len(fields) == 0 {
		return line, nil
	}

	encoder := zapcore.NewMapObjectEncoder()
	addFields(encoder, fields)

	context, err := json.MarshalIndent(encoder.Fields, "", "  ")
	if err != nil {
		return line, err
	}

	line.AppendString(string(context))

	if enc.cfg.LineEnding != "" {
		line.AppendString(enc.cfg.LineEnding)
	} else {
		line.AppendString(zapcore.DefaultLineEnding)
	}

	return line, nil
}

func addFields(enc ObjectEncoder, fields []Field) {
	for i := range fields {
		fields[i].AddTo(enc)
	}
}
