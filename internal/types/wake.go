package types

import "regexp"

// WakeConditionType represents the type of wake condition.
type WakeConditionType string

const (
	WakeConditionOnMention WakeConditionType = "on_mention" // Wake when specific users post
	WakeConditionAfter     WakeConditionType = "after"      // Wake after time delay
	WakeConditionPattern   WakeConditionType = "pattern"    // Wake on regex pattern match
	WakeConditionPrompt    WakeConditionType = "prompt"     // Wake based on LLM evaluation with polling
)

// WakePersistMode controls how wake conditions survive triggers.
type WakePersistMode string

const (
	WakePersistNone           WakePersistMode = ""                     // Default: clears after trigger (one-shot)
	WakePersist               WakePersistMode = "persist"              // Survives trigger, manual clear required
	WakePersistUntilBye       WakePersistMode = "persist_until_bye"    // Survives trigger, auto-clears on bye
	WakePersistRestoreOnBack  WakePersistMode = "persist_restore_back" // Pauses on bye, restores on back
)

// WakeCondition represents a condition that can trigger an agent wake.
type WakeCondition struct {
	GUID           string            `json:"guid"`
	AgentID        string            `json:"agent_id"`                // Agent to wake
	SetBy          string            `json:"set_by"`                  // Agent who set this condition
	Type           WakeConditionType `json:"type"`                    // on_mention, after, pattern, prompt
	Pattern        *string           `json:"pattern,omitempty"`       // Regex pattern for pattern type
	OnAgents       []string          `json:"on_agents,omitempty"`     // Agents to watch for on_mention type
	InThread       *string           `json:"in_thread,omitempty"`     // Scope to specific thread (nil = anywhere except meta/)
	AfterMs        *int64            `json:"after_ms,omitempty"`      // Delay for after type
	UseRouter      bool              `json:"use_router,omitempty"`    // Use haiku router for ambiguous patterns
	Prompt         *string           `json:"prompt,omitempty"`        // Context passed on wake
	PromptText     *string           `json:"prompt_text,omitempty"`   // LLM prompt for prompt type
	PollIntervalMs *int64            `json:"poll_interval_ms,omitempty"` // Poll interval for prompt type
	LastPolledAt   *int64            `json:"last_polled_at,omitempty"` // Last time prompt condition was polled
	PersistMode    WakePersistMode   `json:"persist_mode,omitempty"`  // How condition survives triggers
	Paused         bool              `json:"paused,omitempty"`        // True when condition is paused (for restore-on-back)
	CreatedAt      int64             `json:"created_at"`
	ExpiresAt      *int64            `json:"expires_at,omitempty"`    // For after type
}

// WakeConditionInput represents new wake condition data.
type WakeConditionInput struct {
	AgentID        string            `json:"agent_id"`
	SetBy          string            `json:"set_by"`
	Type           WakeConditionType `json:"type"`
	Pattern        *string           `json:"pattern,omitempty"`
	OnAgents       []string          `json:"on_agents,omitempty"`
	InThread       *string           `json:"in_thread,omitempty"`
	AfterMs        *int64            `json:"after_ms,omitempty"`
	UseRouter      bool              `json:"use_router,omitempty"`
	Prompt         *string           `json:"prompt,omitempty"`
	PromptText     *string           `json:"prompt_text,omitempty"`
	PollIntervalMs *int64            `json:"poll_interval_ms,omitempty"`
	PersistMode    WakePersistMode   `json:"persist_mode,omitempty"`
}

// CompiledPattern holds a pre-compiled regex for efficient matching.
type CompiledPattern struct {
	Condition *WakeCondition
	Regex     *regexp.Regexp
}

// CompilePattern compiles the pattern regex.
// Returns nil if compilation fails or pattern is nil.
func (wc *WakeCondition) CompilePattern() *CompiledPattern {
	if wc.Pattern == nil || wc.Type != WakeConditionPattern {
		return nil
	}

	re, err := regexp.Compile(*wc.Pattern)
	if err != nil {
		return nil
	}

	return &CompiledPattern{
		Condition: wc,
		Regex:     re,
	}
}

// MatchesMessage checks if a message body matches the pattern.
func (cp *CompiledPattern) MatchesMessage(body string) bool {
	if cp == nil || cp.Regex == nil {
		return false
	}
	return cp.Regex.MatchString(body)
}

// MatchesThread checks if the message is in a valid scope for this condition.
// Returns false for meta/ threads unless explicitly scoped there.
func (wc *WakeCondition) MatchesThread(home string) bool {
	// If scoped to specific thread, only match that thread
	if wc.InThread != nil {
		return home == *wc.InThread
	}

	// Default: exclude meta/ hierarchy unless explicitly scoped
	if len(home) >= 5 && home[:5] == "meta/" {
		return false
	}

	return true
}

// WakeRouterPayload is the input to the wake router mlld script.
type WakeRouterPayload struct {
	Message string  `json:"message"` // Message body
	From    string  `json:"from"`    // Who sent it
	Agent   string  `json:"agent"`   // Agent to potentially wake
	Pattern string  `json:"pattern"` // The matched pattern
	Thread  *string `json:"thread"`  // Thread context
}

// WakeRouterResult is the output from the wake router.
type WakeRouterResult struct {
	ShouldWake bool    `json:"shouldWake"`
	Reason     string  `json:"reason,omitempty"`
	Confidence float64 `json:"confidence"`
}

// AgentStatusForPrompt represents agent status for wake prompt evaluation.
type AgentStatusForPrompt struct {
	Name        string  `json:"name"`
	Presence    string  `json:"presence"`
	Status      *string `json:"status,omitempty"`
	IdleSeconds int64   `json:"idle_seconds"`
}

// WakePromptPayload is the input to the wake prompt mlld script.
type WakePromptPayload struct {
	Agent      string                 `json:"agent"`       // Agent that set this condition
	Prompt     string                 `json:"prompt"`      // User-provided prompt/criteria
	Agents     []AgentStatusForPrompt `json:"agents"`      // Current agent statuses
	InThread   *string                `json:"in_thread"`   // Thread scope if any
}

// WakePromptResult is the output from the wake prompt script.
type WakePromptResult struct {
	ShouldWake bool    `json:"shouldWake"`
	Reason     string  `json:"reason,omitempty"`
	Confidence float64 `json:"confidence"`
}

// StdoutRepairPayload is the input to the stdout repair mlld script.
type StdoutRepairPayload struct {
	Stdout   string  `json:"stdout"`   // Captured stdout buffer content
	LastPost *string `json:"lastPost"` // Agent's last fray post (nil if none)
	AgentID  string  `json:"agentId"`  // Agent identifier
}

// StdoutRepairResult is the output from the stdout repair script.
type StdoutRepairResult struct {
	Post    bool   `json:"post"`              // Whether to post the content
	Content string `json:"content,omitempty"` // Cleaned content to post
	Reason  string `json:"reason,omitempty"`  // Why posting/not posting
}
