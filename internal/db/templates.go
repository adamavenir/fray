package db

import (
	_ "embed"
)

//go:embed templates/mentions.mld
var MentionsRouterTemplate []byte

//go:embed templates/status.mld
var StatusTemplate []byte

//go:embed templates/wake-router.mld
var WakeRouterTemplate []byte

//go:embed templates/wake-prompt.mld
var WakePromptTemplate []byte

//go:embed templates/stdout-repair.mld
var StdoutRepairTemplate []byte

// Slash command templates (shipped with fray init)

//go:embed templates/slash/fly.mld
var FlyTemplate []byte

//go:embed templates/slash/land.mld
var LandTemplate []byte

//go:embed templates/slash/hand.mld
var HandTemplate []byte

//go:embed templates/slash/hop.mld
var HopTemplate []byte
