package cmds

import (
	"os"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewHelpCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.
Simply type ` + root.Name() + ` help [path to command] for full details.

Use -all to show full command reference with all subcommands and flags.`,
		Run: func(cmd *cobra.Command, args []string) {
			if helpAll, _ := cmd.Flags().GetBool("all"); helpAll {
				t := template.Must(template.New("").Funcs(funcs).Parse(cmdHelpTemplate))
				t.Execute(os.Stdout, root)
				return
			}
			root.HelpFunc()(root, args)
		},
	}
	cmd.Flags().Bool("all", false, "Show full command reference")
	return cmd
}

var funcs = template.FuncMap{
	"commands": func(cmd *cobra.Command) []*cobra.Command {
		var out []*cobra.Command
		for _, c := range cmd.Commands() {
			if c.IsAvailableCommand() && c.Name() != "help" {
				out = append(out, c)
			}
		}
		return out
	},
	"flags": func(cmd *cobra.Command) []*pflag.Flag {
		var out []*pflag.Flag
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Name != "help" && f.Name != "all" {
				out = append(out, f)
			}
		})
		return out
	},
	"required": func(f *pflag.Flag) bool {
		_, ok := f.Annotations[cobra.BashCompOneRequiredFlag]
		return ok
	},
	"defval": func(f *pflag.Flag) string {
		if f.DefValue == "" || f.DefValue == "false" || f.DefValue == "0" || f.DefValue == "[]" {
			return ""
		}
		return f.DefValue
	},
	"indent": func(n int, s string) string {
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			if line != "" {
				lines[i] = strings.Repeat(" ", n) + line
			}
		}
		return strings.Join(lines, "\n")
	},
}

const cmdHelpTemplate = `
{{- .Name }}{{ with .Short }} - {{ . }}{{ end }}
{{ with .Long }}
{{ . }}
{{ end }}
Usage:
{{ .Name }}
{{- range commands . }}

    {{ .Name }}{{ with .Short }} # {{ . }}{{ end }}
    {{- with .Example }}
        Examples:
{{ . | indent 8 }}
    {{- end }}
    {{- range flags . }}
        {{ template "flag" . }}
    {{- end }}
    {{- range commands . }}

        {{ .Use }}{{ with .Short }} # {{ . }}{{ end }}
        {{- range flags . }}
            {{ template "flag" . }}
        {{- end }}
		{{- with .Example }}
            Examples:
{{ . | indent 12 }}
        {{- end }}
        {{- range commands . }}

            {{ .Use }}{{ with .Short }} # {{ . }}{{ end }}
            {{- with .Example }}
                Examples:
{{ . | indent 16 }}
            {{- end }}
            {{- range flags . }}
                {{ template "flag" . }}
            {{- end }}
        {{- end }}
    {{- end }}
{{- end }}
{{- define "flag" -}}
{{- if required . }}--{{ .Name }} <{{ .Value.Type }}>
{{- else }}[--{{ .Name }} <{{ .Value.Type }}>]
{{- end }}
{{- with .Usage }}  # {{ . }}{{ end }}
{{- with defval . }} (default: {{ . }}){{ end }}
{{- end }}
`
