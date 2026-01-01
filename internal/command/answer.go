package command

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/adamavenir/fray/internal/core"
	"github.com/adamavenir/fray/internal/db"
	"github.com/adamavenir/fray/internal/types"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	answerHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	answerQuestionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	answerOptionStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("157"))
	answerProStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	answerConStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	answerMetaStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	answerPromptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	answerSkipStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

// NewAnswerCmd creates the answer command.
func NewAnswerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "answer [question-id] [answer-text]",
		Short: "Answer questions",
		Long: `Answer questions interactively or directly.

Interactive mode (for humans):
  fray answer              Review and answer all open questions one at a time

Direct mode (for agents):
  fray answer <qstn-id> "answer text" --as agent
                           Answer a specific question directly

In interactive mode:
  - Type a letter (a, b, c) to select a proposed option
  - Type your own answer
  - Press 's' to skip the question for now
  - Press 'q' to quit

Skipped questions are offered for review at the end of the session.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := GetContext(cmd)
			if err != nil {
				return writeCommandError(cmd, err)
			}
			defer ctx.DB.Close()

			agentRef, _ := cmd.Flags().GetString("as")

			// Direct mode: answer <qstn-id> "answer" --as agent
			if len(args) >= 2 {
				if agentRef == "" {
					return writeCommandError(cmd, fmt.Errorf("--as is required for direct answer mode"))
				}
				return runDirectAnswer(ctx, args[0], args[1], agentRef)
			}

			// Interactive mode: answer (uses username from config)
			if len(args) == 0 {
				return runInteractiveAnswer(ctx, agentRef)
			}

			// Single arg could be question ID with missing answer
			return writeCommandError(cmd, fmt.Errorf("usage: fray answer <question-id> \"answer\" --as agent\n       fray answer (interactive mode)"))
		},
	}

	cmd.Flags().StringP("as", "", "", "agent identity (required for direct mode)")
	return cmd
}

// runDirectAnswer handles: fray answer <qstn-id> "answer" --as agent
func runDirectAnswer(ctx *CommandContext, questionRef, answerText, agentRef string) error {
	agentID, err := resolveAgentRef(ctx, agentRef)
	if err != nil {
		return err
	}

	agent, err := db.GetAgent(ctx.DB, agentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found: @%s", agentID)
	}
	if agent.LeftAt != nil {
		return fmt.Errorf("agent @%s has left. Use 'fray back @%s' to resume", agentID, agentID)
	}

	question, err := resolveQuestionRef(ctx.DB, questionRef)
	if err != nil {
		return err
	}

	if question.Status == types.QuestionStatusClosed {
		return fmt.Errorf("question %s is already closed", question.GUID)
	}
	if question.Status == types.QuestionStatusAnswered {
		return fmt.Errorf("question %s is already answered", question.GUID)
	}

	if err := postAnswer(ctx.DB, ctx.Project.DBPath, agentID, *question, answerText, ctx.ProjectConfig); err != nil {
		return err
	}

	if ctx.JSONMode {
		payload := map[string]any{
			"question_id": question.GUID,
			"answered_by": agentID,
			"answer":      answerText,
		}
		return json.NewEncoder(os.Stdout).Encode(payload)
	}

	fmt.Printf("Answered %s\n", question.GUID)
	return nil
}

// runInteractiveAnswer handles: fray answer (interactive mode for humans)
func runInteractiveAnswer(ctx *CommandContext, agentRef string) error {
	// Determine identity: --as flag or username from config
	var identity string
	if agentRef != "" {
		resolved, err := resolveAgentRef(ctx, agentRef)
		if err != nil {
			return err
		}
		identity = resolved
	} else {
		// Use username from config (human user)
		username, err := db.GetConfig(ctx.DB, "username")
		if err != nil {
			return err
		}
		if username == "" {
			return fmt.Errorf("no username configured. Use 'fray chat' first or specify --as")
		}
		identity = username
	}

	// Get open questions addressed to this identity
	questions, err := db.GetQuestions(ctx.DB, &types.QuestionQueryOptions{
		Statuses: []types.QuestionStatus{types.QuestionStatusOpen},
		ToAgent:  &identity,
	})
	if err != nil {
		return err
	}

	if len(questions) == 0 {
		fmt.Printf("No open questions for @%s\n", identity)
		return nil
	}

	return runAnswerSession(ctx.DB, ctx.Project.DBPath, identity, ctx.ProjectConfig, questions)
}

// questionSet groups questions by their source message.
type questionSet struct {
	askedIn   *string
	createdAt int64
	questions []types.Question
}

func runAnswerSession(database *sql.DB, dbPath string, identity string, config *db.ProjectConfig, questions []types.Question) error {
	// Group by asked_in message and sort
	sets := groupQuestionSets(questions)

	reader := bufio.NewReader(os.Stdin)
	var skipped []types.Question
	answered := 0

	for setIdx, set := range sets {
		for qIdx, q := range set.questions {
			result, err := presentQuestion(database, q, reader, setIdx+1, qIdx+1, len(sets), len(set.questions))
			if err != nil {
				return err
			}

			switch result.action {
			case actionAnswer:
				if err := postAnswer(database, dbPath, identity, q, result.answer, config); err != nil {
					return err
				}
				answered++
				fmt.Println(answerMetaStyle.Render("✓ Answer posted\n"))

			case actionSkip:
				skipped = append(skipped, q)
				fmt.Println(answerSkipStyle.Render("→ Skipped\n"))

			case actionQuit:
				printSummary(answered, len(skipped))
				return nil
			}
		}
	}

	// Offer to review skipped questions
	if len(skipped) > 0 {
		fmt.Printf("\n%s\n", answerPromptStyle.Render(fmt.Sprintf("You skipped %d question(s). Review them now? [y/n]: ", len(skipped))))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "y" || input == "yes" {
			for i, q := range skipped {
				result, err := presentQuestion(database, q, reader, 1, i+1, 1, len(skipped))
				if err != nil {
					return err
				}

				switch result.action {
				case actionAnswer:
					if err := postAnswer(database, dbPath, identity, q, result.answer, config); err != nil {
						return err
					}
					answered++
					fmt.Println(answerMetaStyle.Render("✓ Answer posted\n"))

				case actionSkip:
					fmt.Println(answerSkipStyle.Render("→ Skipped again\n"))

				case actionQuit:
					printSummary(answered, len(skipped)-i-1)
					return nil
				}
			}
		}
	}

	printSummary(answered, 0)
	return nil
}

func groupQuestionSets(questions []types.Question) []questionSet {
	// Group by asked_in
	groups := make(map[string]*questionSet)
	var nilGroup *questionSet

	for _, q := range questions {
		key := ""
		if q.AskedIn != nil {
			key = *q.AskedIn
		}

		if key == "" {
			if nilGroup == nil {
				nilGroup = &questionSet{createdAt: q.CreatedAt}
			}
			nilGroup.questions = append(nilGroup.questions, q)
		} else {
			if groups[key] == nil {
				groups[key] = &questionSet{askedIn: q.AskedIn, createdAt: q.CreatedAt}
			}
			groups[key].questions = append(groups[key].questions, q)
		}
	}

	// Convert to slice and sort by creation time (newest first)
	var sets []questionSet
	for _, g := range groups {
		sets = append(sets, *g)
	}
	if nilGroup != nil {
		sets = append(sets, *nilGroup)
	}

	sort.Slice(sets, func(i, j int) bool {
		return sets[i].createdAt > sets[j].createdAt
	})

	return sets
}

type answerAction int

const (
	actionAnswer answerAction = iota
	actionSkip
	actionQuit
)

type answerResult struct {
	action answerAction
	answer string
}

func presentQuestion(database *sql.DB, q types.Question, reader *bufio.Reader, setNum, qNum, totalSets, totalInSet int) (answerResult, error) {
	fmt.Print("\033[H\033[2J") // Clear screen

	// Header
	progress := fmt.Sprintf("Question %d/%d", qNum, totalInSet)
	if totalSets > 1 {
		progress = fmt.Sprintf("Set %d/%d, Question %d/%d", setNum, totalSets, qNum, totalInSet)
	}
	fmt.Println(answerHeaderStyle.Render(progress))
	fmt.Println()

	// Question metadata
	fromTo := fmt.Sprintf("From @%s", q.FromAgent)
	if q.ToAgent != nil {
		fromTo += fmt.Sprintf(" → @%s", *q.ToAgent)
	}
	fmt.Println(answerMetaStyle.Render(fromTo))

	if q.ThreadGUID != nil {
		thread, _ := db.GetThread(database, *q.ThreadGUID)
		if thread != nil {
			fmt.Println(answerMetaStyle.Render(fmt.Sprintf("Thread: %s", thread.Name)))
		}
	}
	fmt.Println()

	// Question text
	fmt.Println(answerQuestionStyle.Render(q.Re))
	fmt.Println()

	// Options with pros/cons
	optionLabels := []string{}
	if len(q.Options) > 0 {
		for i, opt := range q.Options {
			letter := string(rune('a' + i))
			optionLabels = append(optionLabels, letter)
			fmt.Printf("  %s. %s\n", answerOptionStyle.Render(letter), opt.Label)

			for _, pro := range opt.Pros {
				fmt.Printf("     %s %s\n", answerProStyle.Render("+ Pro:"), pro)
			}
			for _, con := range opt.Cons {
				fmt.Printf("     %s %s\n", answerConStyle.Render("- Con:"), con)
			}
			if len(opt.Pros) > 0 || len(opt.Cons) > 0 {
				fmt.Println()
			}
		}
		fmt.Println()
	}

	// Prompt
	if len(optionLabels) > 0 {
		fmt.Printf("%s ", answerPromptStyle.Render(fmt.Sprintf("[%s] select, [s]kip, [q]uit, or type answer:", strings.Join(optionLabels, "/"))))
	} else {
		fmt.Printf("%s ", answerPromptStyle.Render("[s]kip, [q]uit, or type answer:"))
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return answerResult{action: actionQuit}, nil
	}
	input = strings.TrimSpace(input)

	if input == "" {
		return answerResult{action: actionSkip}, nil
	}

	inputLower := strings.ToLower(input)

	if inputLower == "s" || inputLower == "skip" {
		return answerResult{action: actionSkip}, nil
	}

	if inputLower == "q" || inputLower == "quit" {
		return answerResult{action: actionQuit}, nil
	}

	// Check if it's an option selection
	if len(inputLower) == 1 && len(q.Options) > 0 {
		idx := int(inputLower[0] - 'a')
		if idx >= 0 && idx < len(q.Options) {
			return answerResult{action: actionAnswer, answer: q.Options[idx].Label}, nil
		}
	}

	// Custom answer
	return answerResult{action: actionAnswer, answer: input}, nil
}

func postAnswer(database *sql.DB, dbPath string, identity string, q types.Question, answer string, config *db.ProjectConfig) error {
	now := time.Now().Unix()

	// Create the answer message
	bases, _ := db.GetAgentBases(database)
	mentions := core.ExtractMentions(answer, bases)
	mentions = core.ExpandAllMention(mentions, bases)

	home := ""
	if q.ThreadGUID != nil {
		home = *q.ThreadGUID
	}

	created, err := db.CreateMessage(database, types.Message{
		TS:        now,
		FromAgent: identity,
		Body:      answer,
		Mentions:  mentions,
		Home:      home,
	})
	if err != nil {
		return err
	}

	if err := db.AppendMessage(dbPath, created); err != nil {
		return err
	}

	// Update agent last seen (if this is an agent, not a user)
	agent, _ := db.GetAgent(database, identity)
	if agent != nil {
		updates := db.AgentUpdates{LastSeen: types.OptionalInt64{Set: true, Value: &now}}
		_ = db.UpdateAgent(database, identity, updates)
	}

	// Update question status
	statusValue := string(types.QuestionStatusAnswered)
	updated, err := db.UpdateQuestion(database, q.GUID, db.QuestionUpdates{
		Status:     types.OptionalString{Set: true, Value: &statusValue},
		AnsweredIn: types.OptionalString{Set: true, Value: &created.ID},
	})
	if err != nil {
		return err
	}

	if err := db.AppendQuestionUpdate(dbPath, db.QuestionUpdateJSONLRecord{
		GUID:       updated.GUID,
		Status:     &statusValue,
		AnsweredIn: &created.ID,
	}); err != nil {
		return err
	}

	return nil
}

func printSummary(answered, stillSkipped int) {
	if answered == 0 && stillSkipped == 0 {
		fmt.Println("\nNo questions answered.")
		return
	}

	summary := fmt.Sprintf("\nDone! Answered %d question(s)", answered)
	if stillSkipped > 0 {
		summary += fmt.Sprintf(", %d still skipped", stillSkipped)
	}
	summary += "."
	fmt.Println(summary)
}
