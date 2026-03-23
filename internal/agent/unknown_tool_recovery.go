package agent

import (
	"bilge-lib/internal/middleware"
	"context"
	"errors"
	"regexp"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

const maxUnknownToolRecoveryAttempts = 2

var unknownToolPattern = regexp.MustCompile(`tool ([^\s]+) not found in toolsNode indexes`)

type unknownToolRecoveryAgent struct {
	inner          adk.Agent
	resumableInner adk.ResumableAgent
}

func wrapUnknownToolRecoveryAgent(inner adk.Agent) adk.Agent {
	wrapped := &unknownToolRecoveryAgent{inner: inner}
	if resumable, ok := inner.(adk.ResumableAgent); ok {
		wrapped.resumableInner = resumable
	}
	return wrapped
}

func (a *unknownToolRecoveryAgent) Name(ctx context.Context) string {
	return a.inner.Name(ctx)
}

func (a *unknownToolRecoveryAgent) Description(ctx context.Context) string {
	return a.inner.Description(ctx)
}

func (a *unknownToolRecoveryAgent) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()

		currentInput := cloneAgentInput(input)
		for attempts := 0; attempts <= maxUnknownToolRecoveryAttempts; attempts++ {
			innerIter := a.inner.Run(ctx, currentInput, opts...)
			lastToolCall := (*schema.Message)(nil)
			var pendingToolCall <-chan *schema.Message

			recovered := false
			for {
				event, ok := innerIter.Next()
				if !ok {
					return
				}

				if msg, pending := messageFromEvent(event); msg != nil {
					lastToolCall = cloneMessage(msg)
				} else if pending != nil {
					pendingToolCall = pending
				}

				if event.Err == nil {
					gen.Send(event)
					continue
				}

				lastToolCall = resolvePendingToolCall(lastToolCall, pendingToolCall)
				nextInput, recoveryEvent, ok := recoverUnknownToolInput(ctx, currentInput, lastToolCall, event.Err)
				if !ok || attempts == maxUnknownToolRecoveryAttempts {
					gen.Send(event)
					return
				}

				if recoveryEvent != nil {
					gen.Send(recoveryEvent)
				}
				currentInput = nextInput
				recovered = true
				break
			}

			if !recovered {
				return
			}
		}
	}()

	return iter
}

func (a *unknownToolRecoveryAgent) Resume(ctx context.Context, info *adk.ResumeInfo, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	if a.resumableInner == nil {
		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		go func() {
			defer gen.Close()
			gen.Send(&adk.AgentEvent{Err: errors.New("resume is not supported for non-resumable unknown tool recovery agent")})
		}()
		return iter
	}

	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()

		currentInfo := cloneResumeInfo(info)
		for attempts := 0; attempts <= maxUnknownToolRecoveryAttempts; attempts++ {
			innerIter := a.resumableInner.Resume(ctx, currentInfo, opts...)
			lastToolCall := (*schema.Message)(nil)
			var pendingToolCall <-chan *schema.Message

			recovered := false
			for {
				event, ok := innerIter.Next()
				if !ok {
					return
				}

				if msg, pending := messageFromEvent(event); msg != nil {
					lastToolCall = cloneMessage(msg)
				} else if pending != nil {
					pendingToolCall = pending
				}

				if event.Err == nil {
					gen.Send(event)
					continue
				}

				lastToolCall = resolvePendingToolCall(lastToolCall, pendingToolCall)
				nextInfo, recoveryEvent, ok := recoverUnknownToolResumeInfo(ctx, currentInfo, lastToolCall, event.Err)
				if !ok || attempts == maxUnknownToolRecoveryAttempts {
					gen.Send(event)
					return
				}

				if recoveryEvent != nil {
					gen.Send(recoveryEvent)
				}
				currentInfo = nextInfo
				recovered = true
				break
			}

			if !recovered {
				return
			}
		}
	}()

	return iter
}

func recoverUnknownToolInput(ctx context.Context, current *adk.AgentInput, lastToolCall *schema.Message, runErr error) (*adk.AgentInput, *adk.AgentEvent, bool) {
	if lastToolCall == nil || len(lastToolCall.ToolCalls) == 0 || runErr == nil {
		return nil, nil, false
	}

	matches := unknownToolPattern.FindStringSubmatch(runErr.Error())
	if len(matches) != 2 {
		return nil, nil, false
	}

	toolCall := lastToolCall.ToolCalls[0]
	if toolCall.Function.Name != matches[1] {
		return nil, nil, false
	}

	toolResult, err := middleware.UnknownToolHandler()(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
	if err != nil {
		return nil, nil, false
	}

	toolMessage := schema.ToolMessage(toolResult, toolCall.ID, schema.WithToolName(toolCall.Function.Name))
	recoveryEvent := adk.EventFromMessage(toolMessage, nil, schema.Tool, toolCall.Function.Name)

	nextInput := cloneAgentInput(current)
	nextInput.Messages = append(nextInput.Messages, cloneMessage(lastToolCall), toolMessage)
	return nextInput, recoveryEvent, true
}

func recoverUnknownToolResumeInfo(ctx context.Context, current *adk.ResumeInfo, lastToolCall *schema.Message, runErr error) (*adk.ResumeInfo, *adk.AgentEvent, bool) {
	if lastToolCall == nil || len(lastToolCall.ToolCalls) == 0 || runErr == nil {
		return nil, nil, false
	}

	matches := unknownToolPattern.FindStringSubmatch(runErr.Error())
	if len(matches) != 2 {
		return nil, nil, false
	}

	toolCall := lastToolCall.ToolCalls[0]
	if toolCall.Function.Name != matches[1] {
		return nil, nil, false
	}

	toolResult, err := middleware.UnknownToolHandler()(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
	if err != nil {
		return nil, nil, false
	}

	toolMessage := schema.ToolMessage(toolResult, toolCall.ID, schema.WithToolName(toolCall.Function.Name))
	nextInfo, ok := appendResumeHistoryModifier(current, lastToolCall, toolMessage)
	if !ok {
		return nil, nil, false
	}

	recoveryEvent := adk.EventFromMessage(toolMessage, nil, schema.Tool, toolCall.Function.Name)
	return nextInfo, recoveryEvent, true
}

func messageFromEvent(event *adk.AgentEvent) (*schema.Message, <-chan *schema.Message) {
	if event == nil || event.Output == nil || event.Output.MessageOutput == nil {
		return nil, nil
	}
	if event.Output.MessageOutput.IsStreaming {
		return nil, messageFromStreamingEvent(event)
	}
	msg := event.Output.MessageOutput.Message
	if msg == nil || len(msg.ToolCalls) == 0 {
		return nil, nil
	}
	return msg, nil
}

func messageFromStreamingEvent(event *adk.AgentEvent) <-chan *schema.Message {
	if event == nil || event.Output == nil || event.Output.MessageOutput == nil || event.Output.MessageOutput.MessageStream == nil {
		return nil
	}

	streams := event.Output.MessageOutput.MessageStream.Copy(2)
	event.Output.MessageOutput.MessageStream = streams[0]

	msgCh := make(chan *schema.Message, 1)
	go func() {
		defer close(msgCh)
		msg, err := schema.ConcatMessageStream(streams[1])
		if err != nil || msg == nil || len(msg.ToolCalls) == 0 {
			return
		}
		msgCh <- cloneMessage(msg)
	}()

	return msgCh
}

func resolvePendingToolCall(lastToolCall *schema.Message, pendingToolCall <-chan *schema.Message) *schema.Message {
	if lastToolCall != nil || pendingToolCall == nil {
		return lastToolCall
	}
	msg, ok := <-pendingToolCall
	if !ok {
		return nil
	}
	return msg
}

func appendResumeHistoryModifier(current *adk.ResumeInfo, lastToolCall *schema.Message, toolMessage *schema.Message) (*adk.ResumeInfo, bool) {
	var baseHistoryModifier func(ctx context.Context, history []adk.Message) []adk.Message
	if current != nil && current.ResumeData != nil {
		resumeData, ok := current.ResumeData.(*adk.ChatModelAgentResumeData)
		if !ok {
			return nil, false
		}
		if resumeData != nil {
			baseHistoryModifier = resumeData.HistoryModifier
		}
	}

	nextInfo := cloneResumeInfo(current)
	assistantToolCall := cloneMessage(lastToolCall)
	toolResponse := cloneMessage(toolMessage)
	nextInfo.ResumeData = &adk.ChatModelAgentResumeData{
		HistoryModifier: func(ctx context.Context, history []adk.Message) []adk.Message {
			modifiedHistory := cloneMessages(history)
			if baseHistoryModifier != nil {
				modifiedHistory = cloneMessages(baseHistoryModifier(ctx, modifiedHistory))
			}
			return append(modifiedHistory, cloneMessage(assistantToolCall), cloneMessage(toolResponse))
		},
	}

	return nextInfo, true
}

func cloneResumeInfo(info *adk.ResumeInfo) *adk.ResumeInfo {
	if info == nil {
		return &adk.ResumeInfo{}
	}
	cloned := *info
	return &cloned
}

func cloneAgentInput(input *adk.AgentInput) *adk.AgentInput {
	if input == nil {
		return &adk.AgentInput{}
	}
	cloned := &adk.AgentInput{
		EnableStreaming: input.EnableStreaming,
	}
	if len(input.Messages) == 0 {
		return cloned
	}
	cloned.Messages = make([]adk.Message, 0, len(input.Messages))
	for _, msg := range input.Messages {
		cloned.Messages = append(cloned.Messages, cloneMessage(msg))
	}
	return cloned
}

func cloneMessage(msg *schema.Message) *schema.Message {
	if msg == nil {
		return nil
	}
	cloned := *msg
	if len(msg.ToolCalls) > 0 {
		cloned.ToolCalls = append([]schema.ToolCall(nil), msg.ToolCalls...)
	}
	if len(msg.UserInputMultiContent) > 0 {
		cloned.UserInputMultiContent = append([]schema.MessageInputPart(nil), msg.UserInputMultiContent...)
	}
	if len(msg.AssistantGenMultiContent) > 0 {
		cloned.AssistantGenMultiContent = append([]schema.MessageOutputPart(nil), msg.AssistantGenMultiContent...)
	}
	return &cloned
}

func cloneMessages(messages []*schema.Message) []*schema.Message {
	if len(messages) == 0 {
		return nil
	}

	cloned := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		cloned = append(cloned, cloneMessage(msg))
	}
	return cloned
}
