package tui

import (
	"bilge-lib/internal/runtime"
	"strings"
)

type transcriptNodeKind string

const (
	transcriptNodeAgent transcriptNodeKind = "agent"
	transcriptNodeTool  transcriptNodeKind = "tool"
)

type transcriptNode struct {
	ID        string
	ParentID  string
	Kind      transcriptNodeKind
	AgentName string
	Title     string
	Summary   string
	Result    string
	Status    string
	Approval  *runtime.PendingApproval
	Expanded  bool
	Focused   bool
	Children  []string
}

type transcriptTree struct {
	roots          []string
	nodes          map[string]*transcriptNode
	toolByCallID   map[string]string
	lastAgentByRun map[runtime.RunID]string
	toolOrderByRun map[runtime.RunID][]string
}

func newTranscriptTree() *transcriptTree {
	return &transcriptTree{
		nodes:          make(map[string]*transcriptNode),
		toolByCallID:   make(map[string]string),
		lastAgentByRun: make(map[runtime.RunID]string),
		toolOrderByRun: make(map[runtime.RunID][]string),
	}
}

func (t *transcriptTree) ApplyEvent(event runtime.Event) {
	leaf := t.ensureAgentLeaf(event.RunID, event.Payload)
	if leaf != nil && event.Status != "" {
		leaf.Status = string(event.Status)
	}
	if leaf != nil && event.Text != "" && event.Payload != nil && event.Payload.Role == "assistant" {
		leaf.Summary += event.Text
	}

	if event.Payload != nil {
		for _, call := range event.Payload.ToolCalls {
			t.applyToolCall(event.RunID, leaf, call)
		}
		if event.Payload.ToolResult != nil {
			t.applyToolResult(event.RunID, leaf, event.Payload.ToolResult)
		}
	}

	if event.Approval != nil {
		t.applyApproval(event.RunID, leaf, event.Approval)
	}
}

func (t *transcriptTree) ensureAgentLeaf(runID runtime.RunID, payload *runtime.EventPayload) *transcriptNode {
	if payload == nil {
		return t.nodeForID(t.lastAgentByRun[runID])
	}

	path := payload.RunPath
	if len(path) == 0 && payload.AgentName != "" {
		path = []string{payload.AgentName}
	}
	if len(path) == 0 {
		return t.nodeForID(t.lastAgentByRun[runID])
	}

	var parentID string
	var pathParts []string
	var leaf *transcriptNode

	for _, name := range path {
		if name == "" {
			continue
		}
		pathParts = append(pathParts, name)
		nodeID := agentNodeID(runID, pathParts)
		node := t.nodes[nodeID]
		if node == nil {
			node = &transcriptNode{
				ID:        nodeID,
				ParentID:  parentID,
				Kind:      transcriptNodeAgent,
				AgentName: name,
				Title:     name,
			}
			t.nodes[nodeID] = node
			t.attachChild(parentID, nodeID)
		}
		leaf = node
		parentID = nodeID
	}

	if leaf != nil {
		t.lastAgentByRun[runID] = leaf.ID
	}
	return leaf
}

func (t *transcriptTree) applyToolCall(runID runtime.RunID, parent *transcriptNode, call runtime.ToolCallPayload) {
	if call.ID == "" {
		return
	}

	nodeID := toolNodeID(runID, call.ID)
	node := t.nodes[nodeID]
	if node == nil {
		node = &transcriptNode{
			ID:       nodeID,
			Kind:     transcriptNodeTool,
			Title:    call.Name,
			Summary:  call.Arguments,
			Status:   string(runtime.RunStatusRunning),
			ParentID: parentID(parent),
		}
		t.nodes[nodeID] = node
		t.attachChild(node.ParentID, nodeID)
		t.toolOrderByRun[runID] = append(t.toolOrderByRun[runID], nodeID)
	} else {
		if call.Name != "" {
			node.Title = call.Name
		}
		if call.Arguments != "" {
			node.Summary = call.Arguments
		}
		if node.Status == "" {
			node.Status = string(runtime.RunStatusRunning)
		}
	}

	t.toolByCallID[toolLookupKey(runID, call.ID)] = nodeID
}

func (t *transcriptTree) applyToolResult(runID runtime.RunID, parent *transcriptNode, result *runtime.ToolResultPayload) {
	if result == nil {
		return
	}

	node := t.findToolNode(runID, result.ToolCallID, result.ToolName)
	if node == nil {
		nodeID := toolNodeID(runID, result.ToolCallID)
		if result.ToolCallID == "" {
			nodeID = syntheticToolNodeID(runID, result.ToolName)
		}
		node = &transcriptNode{
			ID:       nodeID,
			ParentID: parentID(parent),
			Kind:     transcriptNodeTool,
			Title:    result.ToolName,
		}
		t.nodes[nodeID] = node
		t.attachChild(node.ParentID, nodeID)
		t.toolOrderByRun[runID] = append(t.toolOrderByRun[runID], nodeID)
		if result.ToolCallID != "" {
			t.toolByCallID[toolLookupKey(runID, result.ToolCallID)] = nodeID
		}
	}

	if result.ToolName != "" {
		node.Title = result.ToolName
	}
	node.Result = result.Content
	node.Status = string(runtime.RunStatusCompleted)
}

func (t *transcriptTree) applyApproval(runID runtime.RunID, parent *transcriptNode, approval *runtime.PendingApproval) {
	node := t.findToolNode(runID, "", approval.ToolName)
	if node == nil {
		nodeID := syntheticToolNodeID(runID, approval.ToolName)
		node = &transcriptNode{
			ID:       nodeID,
			ParentID: parentID(parent),
			Kind:     transcriptNodeTool,
			Title:    approval.ToolName,
			Summary:  approval.Arguments,
		}
		t.nodes[nodeID] = node
		t.attachChild(node.ParentID, nodeID)
		t.toolOrderByRun[runID] = append(t.toolOrderByRun[runID], nodeID)
	}

	node.Status = string(runtime.RunStatusInterrupted)
	node.Approval = approval
	if node.Summary == "" {
		node.Summary = approval.Arguments
	}
}

func (t *transcriptTree) findToolNode(runID runtime.RunID, callID, toolName string) *transcriptNode {
	if callID != "" {
		if nodeID, ok := t.toolByCallID[toolLookupKey(runID, callID)]; ok {
			return t.nodeForID(nodeID)
		}
	}

	for i := len(t.toolOrderByRun[runID]) - 1; i >= 0; i-- {
		node := t.nodeForID(t.toolOrderByRun[runID][i])
		if node == nil {
			continue
		}
		if toolName == "" || node.Title == toolName {
			return node
		}
	}

	return nil
}

func (t *transcriptTree) attachChild(parentID, childID string) {
	if childID == "" {
		return
	}
	if parentID == "" {
		for _, existing := range t.roots {
			if existing == childID {
				return
			}
		}
		t.roots = append(t.roots, childID)
		return
	}

	parent := t.nodes[parentID]
	if parent == nil {
		return
	}
	for _, existing := range parent.Children {
		if existing == childID {
			return
		}
	}
	parent.Children = append(parent.Children, childID)
}

func (t *transcriptTree) nodeForID(nodeID string) *transcriptNode {
	if nodeID == "" {
		return nil
	}
	return t.nodes[nodeID]
}

func (t *transcriptTree) runRoots(runID runtime.RunID) []string {
	var roots []string
	prefix := "agent:" + string(runID) + ":"
	for _, nodeID := range t.roots {
		if strings.HasPrefix(nodeID, prefix) {
			roots = append(roots, nodeID)
		}
	}
	return roots
}

func (t *transcriptTree) anyCollapsedTool() bool {
	for _, node := range t.nodes {
		if node.Kind == transcriptNodeTool && !node.Expanded {
			return true
		}
	}
	return false
}

func (t *transcriptTree) setAllToolsExpanded(expanded bool) {
	for _, node := range t.nodes {
		if node.Kind == transcriptNodeTool {
			node.Expanded = expanded
		}
	}
}

func (t *transcriptTree) resolveApproval(runID runtime.RunID, toolName, status string) {
	node := t.findToolNode(runID, "", toolName)
	if node == nil {
		return
	}
	node.Status = status
	node.Approval = nil
}

func (t *transcriptTree) agentChain(nodeID string) []*transcriptNode {
	node := t.nodeForID(nodeID)
	if node == nil {
		return nil
	}

	var chain []*transcriptNode
	for node != nil {
		chain = append(chain, node)
		node = t.nodeForID(node.ParentID)
	}

	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

func (t *transcriptTree) toolIDsForMessages(msgs []Message) []string {
	seen := make(map[string]struct{})
	ids := make([]string, 0, len(t.nodes))
	for _, msg := range msgs {
		if msg.Kind != MessageRun {
			continue
		}
		for _, nodeID := range t.toolOrderByRun[msg.RunID] {
			if _, ok := seen[nodeID]; ok {
				continue
			}
			if node := t.nodeForID(nodeID); node != nil && node.Kind == transcriptNodeTool && t.isVisibleTranscriptTool(node) {
				ids = append(ids, nodeID)
				seen[nodeID] = struct{}{}
			}
		}
	}
	return ids
}

func (t *transcriptTree) setFocusedTool(nodeID string) {
	for _, node := range t.nodes {
		if node.Kind == transcriptNodeTool {
			node.Focused = node.ID == nodeID && nodeID != ""
		}
	}
}

func (t *transcriptTree) isVisibleTranscriptTool(node *transcriptNode) bool {
	if node == nil || hideToolFromTranscript(node) {
		return false
	}
	parent := t.nodeForID(node.ParentID)
	for parent != nil {
		if _, ok := userFacingAgentName(parent); ok {
			return true
		}
		parent = t.nodeForID(parent.ParentID)
	}
	return false
}

func agentNodeID(runID runtime.RunID, path []string) string {
	return "agent:" + string(runID) + ":" + strings.Join(path, "/")
}

func toolNodeID(runID runtime.RunID, callID string) string {
	return "tool:" + string(runID) + ":" + callID
}

func syntheticToolNodeID(runID runtime.RunID, toolName string) string {
	name := toolName
	if name == "" {
		name = "tool_call"
	}
	return "tool:" + string(runID) + ":pending:" + name
}

func toolLookupKey(runID runtime.RunID, callID string) string {
	return string(runID) + ":" + callID
}

func parentID(node *transcriptNode) string {
	if node == nil {
		return ""
	}
	return node.ID
}
