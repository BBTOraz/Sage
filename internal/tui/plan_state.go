package tui

import (
	"bilge-lib/internal/runtime"
	"encoding/json"
	"strings"
)

type planItemStatus string

const (
	planItemQueued planItemStatus = "queued"
	planItemActive planItemStatus = "active"
	planItemDone   planItemStatus = "done"
)

type planItem struct {
	Text   string
	Status planItemStatus
}

type runPlanState struct {
	RunID  runtime.RunID
	Items  []planItem
	Status runtime.RunStatus
}

type planStore struct {
	plans     map[runtime.RunID]*runPlanState
	lastRunID runtime.RunID
}

func newPlanStore() *planStore {
	return &planStore{
		plans: make(map[runtime.RunID]*runPlanState),
	}
}

func (s *planStore) ApplyEvent(event runtime.Event) {
	if s == nil {
		return
	}

	plan := s.ensurePlan(event.RunID)
	s.lastRunID = event.RunID
	plan.Status = event.Status

	if event.Type == runtime.EventRunCompleted {
		markActiveItemsDone(plan)
		return
	}
	if event.Payload == nil {
		return
	}

	for _, call := range event.Payload.ToolCalls {
		switch call.Name {
		case "plan":
			steps := parsePlanStepsFromContent(call.Arguments)
			if len(steps) > 0 {
				s.applyPlanSteps(event.RunID, steps)
			}
		case "write_todos":
			items := parseTodoItemsFromContent(call.Arguments)
			if len(items) > 0 {
				s.applyTodoItems(event.RunID, items)
			}
		case "respond":
			markActiveItemsDone(plan)
		}
	}

	if event.Payload.ToolResult == nil {
		return
	}

	switch event.Payload.ToolResult.ToolName {
	case "plan":
		steps := parsePlanStepsFromContent(event.Payload.ToolResult.Content)
		if len(steps) > 0 {
			s.applyPlanSteps(event.RunID, steps)
		}
	case "write_todos":
		items := parseTodoItemsFromContent(event.Payload.ToolResult.Content)
		if len(items) > 0 {
			s.applyTodoItems(event.RunID, items)
		}
	case "respond":
		markActiveItemsDone(plan)
	}
}

func (s *planStore) planFor(runID runtime.RunID) *runPlanState {
	if s == nil || runID == "" {
		return nil
	}
	plan := s.plans[runID]
	if plan == nil || len(plan.Items) == 0 {
		return nil
	}
	return plan
}

func (s *planStore) ensurePlan(runID runtime.RunID) *runPlanState {
	if runID == "" {
		return &runPlanState{}
	}
	plan := s.plans[runID]
	if plan == nil {
		plan = &runPlanState{RunID: runID}
		s.plans[runID] = plan
	}
	return plan
}

func (s *planStore) applyPlanSteps(runID runtime.RunID, steps []string) {
	plan := s.ensurePlan(runID)
	markActiveItemsDone(plan)
	done := doneItems(plan.Items)

	next := make([]planItem, 0, len(done)+len(steps))
	next = append(next, done...)
	for _, step := range steps {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		next = append(next, planItem{
			Text:   step,
			Status: planItemQueued,
		})
	}
	activateFirstQueued(next)
	plan.Items = next
}

func (s *planStore) applyTodoItems(runID runtime.RunID, items []planItem) {
	plan := s.ensurePlan(runID)
	normalized := make([]planItem, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		normalized = append(normalized, planItem{
			Text:   text,
			Status: item.Status,
		})
	}
	ensureSingleActive(normalized)
	plan.Items = normalized
}

func doneItems(items []planItem) []planItem {
	out := make([]planItem, 0, len(items))
	for _, item := range items {
		if item.Status == planItemDone {
			out = append(out, item)
		}
	}
	return out
}

func markActiveItemsDone(plan *runPlanState) {
	if plan == nil {
		return
	}
	for i := range plan.Items {
		if plan.Items[i].Status == planItemActive {
			plan.Items[i].Status = planItemDone
		}
	}
}

func activateFirstQueued(items []planItem) {
	for i := range items {
		if items[i].Status == planItemQueued {
			items[i].Status = planItemActive
			return
		}
	}
}

func ensureSingleActive(items []planItem) {
	activeSeen := false
	for i := range items {
		switch items[i].Status {
		case planItemActive:
			if activeSeen {
				items[i].Status = planItemQueued
				continue
			}
			activeSeen = true
		case planItemDone, planItemQueued:
		default:
			items[i].Status = planItemQueued
		}
	}
	if !activeSeen {
		activateFirstQueued(items)
	}
}

func parsePlanSteps(raw string) []string {
	var payload struct {
		Steps []string `json:"steps"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return payload.Steps
}

func parsePlanStepsFromContent(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if steps := parsePlanSteps(raw); len(steps) > 0 {
		return steps
	}
	fragment := extractJSONFragment(raw)
	if fragment == "" || fragment == raw {
		return nil
	}
	return parsePlanSteps(fragment)
}

func parseTodoItemsFromContent(raw string) []planItem {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if items := parseTodoItems(raw); len(items) > 0 {
		return items
	}
	fragment := extractJSONFragment(raw)
	if fragment == "" || fragment == raw {
		return nil
	}
	return parseTodoItems(fragment)
}

func parseTodoItems(raw string) []planItem {
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return parseTodoValue(payload)
}

func parseTodoValue(payload any) []planItem {
	switch typed := payload.(type) {
	case map[string]any:
		for _, key := range []string{"todos", "items"} {
			value, ok := typed[key]
			if !ok {
				continue
			}
			items := parseTodoList(value)
			if len(items) > 0 {
				return items
			}
		}

		if value, ok := typed["steps"]; ok {
			return parseStringStepList(value)
		}
	case []any:
		return parseTodoList(typed)
	}

	return nil
}

func parseTodoList(value any) []planItem {
	list, ok := value.([]any)
	if !ok {
		return nil
	}

	items := make([]planItem, 0, len(list))
	for _, item := range list {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		text := stringFromAny(object["content"])
		if text == "" {
			text = stringFromAny(object["text"])
		}
		if text == "" {
			text = stringFromAny(object["title"])
		}
		if text == "" {
			text = stringFromAny(object["step"])
		}
		if text == "" {
			continue
		}
		items = append(items, planItem{
			Text:   text,
			Status: todoStatusFromString(stringFromAny(object["status"])),
		})
	}
	return items
}

func parseStringStepList(value any) []planItem {
	list, ok := value.([]any)
	if !ok {
		return nil
	}

	items := make([]planItem, 0, len(list))
	for _, item := range list {
		text := strings.TrimSpace(stringFromAny(item))
		if text == "" {
			continue
		}
		items = append(items, planItem{Text: text, Status: planItemQueued})
	}
	activateFirstQueued(items)
	return items
}

func todoStatusFromString(raw string) planItemStatus {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "done", "completed", "complete":
		return planItemDone
	case "active", "in_progress", "running", "current":
		return planItemActive
	default:
		return planItemQueued
	}
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func extractJSONFragment(raw string) string {
	start := strings.IndexAny(raw, "[{")
	if start < 0 {
		return ""
	}
	return strings.TrimSpace(raw[start:])
}
