package gql

import (
	"context"
	"net/http"
	"testing"

	gqlclient "github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestChannelSettingsCloakingModeSchemaContract(t *testing.T) {
	server := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: &Resolver{}}))
	client := gqlclient.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	}))

	var resp struct {
		ChannelSettings struct {
			Fields []struct {
				Name string `json:"name"`
				Type struct {
					Name *string `json:"name"`
					Kind string  `json:"kind"`
				} `json:"type"`
			} `json:"fields"`
		} `json:"channelSettings"`
		ChannelSettingsInput struct {
			InputFields []struct {
				Name string `json:"name"`
				Type struct {
					Name *string `json:"name"`
					Kind string  `json:"kind"`
				} `json:"type"`
			} `json:"inputFields"`
		} `json:"channelSettingsInput"`
	}

	client.MustPost(`
		query ChannelSettingsSchema {
			channelSettings: __type(name: "ChannelSettings") {
				fields {
					name
					type {
						name
						kind
					}
				}
			}
			channelSettingsInput: __type(name: "ChannelSettingsInput") {
				inputFields {
					name
					type {
						name
						kind
					}
				}
			}
		}
	`, &resp)

	require.Contains(t, toFieldTypeMap(resp.ChannelSettings.Fields), "cloakingMode")
	require.Equal(t, "CloakingMode", toFieldTypeMap(resp.ChannelSettings.Fields)["cloakingMode"])

	require.Contains(t, toInputFieldTypeMap(resp.ChannelSettingsInput.InputFields), "cloakingMode")
	require.Equal(t, "CloakingMode", toInputFieldTypeMap(resp.ChannelSettingsInput.InputFields)["cloakingMode"])
}

func TestChannelSettingsCloakingModeResolverMapping(t *testing.T) {
	ctx := context.Background()
	resolver := &Resolver{}

	outResolver := &channelSettingsResolver{Resolver: resolver}
	mode := "always"
	settings := &objects.ChannelSettings{CloakingMode: &mode}

	gqlMode, err := outResolver.CloakingMode(ctx, settings)
	require.NoError(t, err)
	require.NotNil(t, gqlMode)
	require.Equal(t, CloakingModeAlways, *gqlMode)

	inResolver := &channelSettingsInputResolver{Resolver: resolver}
	var inputSettings objects.ChannelSettings
	inputMode := CloakingModeFollowGlobal

	err = inResolver.CloakingMode(ctx, &inputSettings, &inputMode)
	require.NoError(t, err)
	require.NotNil(t, inputSettings.CloakingMode)
	require.Equal(t, "follow_global", *inputSettings.CloakingMode)

	err = inResolver.CloakingMode(ctx, &inputSettings, nil)
	require.NoError(t, err)
	require.Nil(t, inputSettings.CloakingMode)
}

func toFieldTypeMap(fields []struct {
	Name string `json:"name"`
	Type struct {
		Name *string `json:"name"`
		Kind string  `json:"kind"`
	} `json:"type"`
}) map[string]string {
	result := make(map[string]string, len(fields))
	for _, field := range fields {
		if field.Type.Name != nil {
			result[field.Name] = *field.Type.Name
			continue
		}
		result[field.Name] = field.Type.Kind
	}

	return result
}

func toInputFieldTypeMap(fields []struct {
	Name string `json:"name"`
	Type struct {
		Name *string `json:"name"`
		Kind string  `json:"kind"`
	} `json:"type"`
}) map[string]string {
	result := make(map[string]string, len(fields))
	for _, field := range fields {
		if field.Type.Name != nil {
			result[field.Name] = *field.Type.Name
			continue
		}
		result[field.Name] = field.Type.Kind
	}

	return result
}
