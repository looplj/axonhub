package conf

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

var (
	_TypeTextUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()
	_TypeDuration        = reflect.TypeFor[time.Duration]()
)

type ViperLoaderOptions struct {
	ConfigName      string
	ConfigType      string
	SearchPaths     []string
	AllowMissing    bool
	EnvPrefix       string
	EnvKeyReplacer  *strings.Replacer
	UnmarshalTag    string
	DefaultFileName string
	SetDefaults     func(v *viper.Viper)
}

type ViperLoader[T any] struct {
	opts ViperLoaderOptions
}

func NewViperLoader[T any](opts ViperLoaderOptions) *ViperLoader[T] {
	if opts.ConfigName == "" {
		opts.ConfigName = "config"
	}
	if opts.ConfigType == "" {
		opts.ConfigType = "yml"
	}
	if opts.DefaultFileName == "" {
		opts.DefaultFileName = fmt.Sprintf("%s.%s", opts.ConfigName, opts.ConfigType)
	}
	if opts.UnmarshalTag == "" {
		opts.UnmarshalTag = "mapstructure"
	}
	return &ViperLoader[T]{opts: opts}
}

func (l *ViperLoader[T]) Load(_ context.Context) (LoadResult[T], error) {
	v := viper.New()
	v.SetConfigName(l.opts.ConfigName)
	v.SetConfigType(l.opts.ConfigType)
	for _, p := range l.opts.SearchPaths {
		v.AddConfigPath(p)
	}

	if l.opts.EnvPrefix != "" {
		v.AutomaticEnv()
		v.SetEnvPrefix(l.opts.EnvPrefix)
		if l.opts.EnvKeyReplacer != nil {
			v.SetEnvKeyReplacer(l.opts.EnvKeyReplacer)
		}
	}

	if l.opts.SetDefaults != nil {
		l.opts.SetDefaults(v)
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			if !l.opts.AllowMissing {
				return LoadResult[T]{ConfigFile: l.defaultConfigFile()}, err
			}
		} else {
			return LoadResult[T]{ConfigFile: l.defaultConfigFile()}, fmt.Errorf("failed to read settings: %w", err)
		}
	}

	var cfg T
	if err := v.Unmarshal(&cfg, func(dc *mapstructure.DecoderConfig) {
		dc.TagName = l.opts.UnmarshalTag
		dc.DecodeHook = customizedDecodeHook
	}); err != nil {
		return LoadResult[T]{ConfigFile: l.configFileUsedOrDefault(v)}, fmt.Errorf("failed to parse settings: %w", err)
	}

	return LoadResult[T]{
		Value:      cfg,
		ConfigFile: l.configFileUsedOrDefault(v),
	}, nil
}

func (l *ViperLoader[T]) DetectConfigFile() (string, error) {
	v := viper.New()
	v.SetConfigName(l.opts.ConfigName)
	v.SetConfigType(l.opts.ConfigType)
	for _, p := range l.opts.SearchPaths {
		v.AddConfigPath(p)
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return l.defaultConfigFile(), nil
		}
		return "", fmt.Errorf("failed to detect config file: %w", err)
	}

	return l.configFileUsedOrDefault(v), nil
}

func (l *ViperLoader[T]) configFileUsedOrDefault(v *viper.Viper) string {
	used := strings.TrimSpace(v.ConfigFileUsed())
	if used != "" {
		return used
	}
	return l.defaultConfigFile()
}

func (l *ViperLoader[T]) defaultConfigFile() string {
	if len(l.opts.SearchPaths) == 0 {
		return l.opts.DefaultFileName
	}
	return filepath.Join(l.opts.SearchPaths[0], l.opts.DefaultFileName)
}

func customizedDecodeHook(srcType reflect.Type, dstType reflect.Type, data any) (any, error) {
	str, ok := data.(string)
	if !ok {
		return data, nil
	}

	switch {
	case reflect.PointerTo(dstType).Implements(_TypeTextUnmarshaler):
		value := reflect.New(dstType)

		u, _ := value.Interface().(encoding.TextUnmarshaler)
		if err := u.UnmarshalText([]byte(str)); err != nil {
			return nil, err
		}

		return u, nil
	case dstType == _TypeDuration:
		if strings.TrimSpace(str) == "" {
			return time.Duration(0), nil
		}
		return time.ParseDuration(str)
	default:
		return data, nil
	}
}
