package agens

import "context"

type contextKey string

const (
	gatewayIDKey contextKey = "agens:gateway_id"
	sessionIDKey contextKey = "agens:session_id"
	senderIDKey  contextKey = "agens:sender_id"
	agentIDKey   contextKey = "agens:agent_id"

	statelessKey   contextKey = "agens:stateless"
	agentAsToolKey contextKey = "agens:agent_as_tool"
)

func stringFromContext(ctx context.Context, key contextKey) (string, bool) {
	v := ctx.Value(key)
	if v == nil {
		return "", false
	}

	str, ok := v.(string)
	if ok {
		return str, true
	}
	return "", false
}

func boolFromContext(ctx context.Context, key contextKey) (bool, bool) {
	v := ctx.Value(key)
	if v == nil {
		return false, false
	}

	value, ok := v.(bool)
	if ok {
		return value, true
	}
	return false, false
}

// GatewayIDFromContext extracts the platform or gateway ID from the context.
func GatewayIDFromContext(ctx context.Context) (string, bool) {
	return stringFromContext(ctx, gatewayIDKey)
}

// SessionIDFromContext extracts the logic conversation space or session ID from the context.
func SessionIDFromContext(ctx context.Context) (string, bool) {
	return stringFromContext(ctx, sessionIDKey)
}

// SenderIDFromContext extracts the entity ID (user, system, agent) that initiated the call.
func SenderIDFromContext(ctx context.Context) (string, bool) {
	return stringFromContext(ctx, senderIDKey)
}

// AgentIDFromContext extracts the calling agent's ID if the execution context belongs to another agent.
func AgentIDFromContext(ctx context.Context) (string, bool) {
	return stringFromContext(ctx, agentIDKey)
}

// StatelessFromContext checks if the current execution context explicitly disables history memory tracking.
func StatelessFromContext(ctx context.Context) (stateless bool, ok bool) {
	return boolFromContext(ctx, statelessKey)
}

// AgentAsToolFromContext checks if the current context was initiated by an agent execution acting as a tool.
func AgentAsToolFromContext(ctx context.Context) (asTool bool, ok bool) {
	return boolFromContext(ctx, agentAsToolKey)
}

// ContextWithGatewayID injects a gateway ID into the context.
func ContextWithGatewayID(ctx context.Context, gateway string) context.Context {
	return context.WithValue(ctx, gatewayIDKey, gateway)
}

// ContextWithSessionID injects a session ID into the context to track conversation state.
func ContextWithSessionID(ctx context.Context, session string) context.Context {
	return context.WithValue(ctx, sessionIDKey, session)
}

// ContextWithSenderID injects the initiator's sender ID into the context.
func ContextWithSenderID(ctx context.Context, sender string) context.Context {
	return context.WithValue(ctx, senderIDKey, sender)
}

// ContextWithAgentID injects the current calling agent's identity into the context.
func ContextWithAgentID(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, agentIDKey, agentID)
}

// ContextWithStateless sets whether the downstream agent should ignore conversation history.
func ContextWithStateless(ctx context.Context, stateless bool) context.Context {
	return context.WithValue(ctx, statelessKey, stateless)
}

// ContextWithAgentAsTool marks the downstream context as an active execution of an agent acting as a tool.
func ContextWithAgentAsTool(ctx context.Context, asTool bool) context.Context {
	return context.WithValue(ctx, agentAsToolKey, asTool)
}
