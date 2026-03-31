package skill

import "embed"

//go:embed SKILL.md
//go:embed references/commands.md
//go:embed references/workspaces.md
var SkillFS embed.FS
