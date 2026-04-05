<skill name="{{.Name}}">
<skill_dir>{{if .Dir}}{{.Dir}}{{else}}BUILTIN_SKILL_NO_REFERENCE{{end}}</skill_dir>

{{if .Dir}}
When this skill references any files (templates, scripts, resources, references, etc.), you MUST use absolute paths based on the skill directory above. For example, if the skill mentions "templates/viewer.html", read it as "{{.Dir}}/viewer.html".
{{end}}

{{.Content}}
</skill>
