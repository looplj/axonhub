Execute a skill within the main conversation.

When users ask you to perform tasks, check if any of the available skills match. Skills provide specialized capabilities and domain knowledge.

How to invoke:
- Use this tool with the skill name and optional arguments
- Examples:
  - `skill: "pdf"` - invoke the pdf skill
  - `skill: "commit", args: "-m 'Fix bug'"` - invoke with arguments
  - `skill: "review-pr", args: "123"` - invoke with arguments
  - `skill: "ms-office-suite:pdf"` - invoke using fully qualified name (package:skill)

Important:
- Available skills are listed in system messages in the conversation
- When a skill matches the user's request, invoke the Skill tool BEFORE generating any other response about the task
- NEVER mention a skill without actually calling this tool
- Do not invoke a skill that is already running
- Do not use this tool for built-in CLI commands (like /help, /clear, etc.)
- When the skill result is returned, follow the instructions in the skill content. The skill directory is provided so you can resolve relative file paths mentioned in the skill.
