package cmds

import (
	"fmt"
	"os"
	"strings"

	"github.com/looplj/axonhub/axon/api"
	"github.com/spf13/cobra"

	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

var ValidModelTypes = []string{
	"chat",
	"embedding",
	"rerank",
	"image_generation",
	"video_generation",
}

func IsValidModelType(t string) bool {
	for _, vt := range ValidModelTypes {
		if strings.EqualFold(vt, t) {
			return true
		}
	}

	return false
}

type ModelFilter struct {
	Type    string
	Keyword string
}

func (f *ModelFilter) Validate() error {
	if f.Type != "" && !IsValidModelType(f.Type) {
		return fmt.Errorf("invalid type %q, valid types: %s", f.Type, strings.Join(ValidModelTypes, ", "))
	}

	return nil
}

func FilterModels(models []*api.AvailableModelsAvailableModelsAvailableModel, filter ModelFilter) []*api.AvailableModelsAvailableModelsAvailableModel {
	filterType := strings.TrimSpace(strings.ToLower(filter.Type))
	keyword := strings.TrimSpace(strings.ToLower(filter.Keyword))

	if filterType == "" && keyword == "" {
		return models
	}

	filtered := make([]*api.AvailableModelsAvailableModelsAvailableModel, 0)

	for _, m := range models {
		if filterType != "" && strings.ToLower(m.Type) != filterType {
			continue
		}

		if keyword != "" {
			idLower := strings.ToLower(m.Id)

			nameLower := strings.ToLower(m.Name)
			if !strings.Contains(idLower, keyword) && !strings.Contains(nameLower, keyword) {
				continue
			}
		}

		filtered = append(filtered, m)
	}

	return filtered
}

func NewModelsCommand(opts StdioOptions) *cobra.Command {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	var (
		filterType string
		keyword    string
	)

	cmd := &cobra.Command{
		Use:   "models",
		Short: "List available models from the server",
		Long: fmt.Sprintf(`List available models from the AxonHub server.

Supports filtering by type and keyword.

Valid types: %s

Examples:
  axonclaw models
  axonclaw models --type chat
  axonclaw models --keyword gpt
  axonclaw models --type chat --keyword claude`, strings.Join(ValidModelTypes, ", ")),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := conf.LoadConfig()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.BaseURL == "" || cfg.APIKey == "" {
				return fmt.Errorf("base_url and api_key must be configured (use 'axonclaw conf set')")
			}

			filter := ModelFilter{Type: filterType, Keyword: keyword}
			if err := filter.Validate(); err != nil {
				return err
			}

			client := api.NewClient(cfg.BaseURL, cfg.APIKey)

			ctx := cmd.Context()

			resp, err := api.AvailableModels(ctx, client)
			if err != nil {
				return err
			}

			models := resp.AvailableModels
			if len(models) == 0 {
				fmt.Fprintln(stdout, "No models found.")
				return nil
			}

			models = FilterModels(models, filter)

			if len(models) == 0 {
				fmt.Fprintln(stdout, "No models match the filter criteria.")
				return nil
			}

			fmt.Fprintf(stdout, "Found %d model(s):\n\n", len(models))

			for _, m := range models {
				fmt.Fprintf(stdout, "- ID: %s\n", m.Id)
				fmt.Fprintf(stdout, "  Name: %s\n", m.Name)
				fmt.Fprintf(stdout, "  Type: %s\n", m.Type)

				if m.Description != nil && *m.Description != "" {
					fmt.Fprintf(stdout, "  Description: %s\n", *m.Description)
				}

				if m.Capabilities != nil && m.Capabilities.Vision {
					fmt.Fprintf(stdout, "  Vision: true\n")
				}

				if m.Pricing != nil {
					fmt.Fprintf(stdout, "  Pricing: input=$%.6f/output=$%.6f", m.Pricing.Input, m.Pricing.Output)

					if m.Pricing.CacheRead > 0 || m.Pricing.CacheWrite > 0 {
						fmt.Fprintf(stdout, " (cache: read=$%.6f/write=$%.6f)", m.Pricing.CacheRead, m.Pricing.CacheWrite)
					}

					fmt.Fprintln(stdout)
				}

				fmt.Fprintln(stdout)
			}

			return nil
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	cmd.Flags().StringVar(&filterType, "type", "", fmt.Sprintf("Filter by model type (%s)", strings.Join(ValidModelTypes, ", ")))
	cmd.Flags().StringVar(&keyword, "keyword", "", "Filter by keyword in model ID or name")

	return cmd
}
