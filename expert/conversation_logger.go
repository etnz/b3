package expert

// ConversationLogger is an interface for logging the interactions
// between an AI, its experts, and its tools. An implementation of this
// interface is passed to an expert at startup to handle the output.
type ConversationLogger interface {
	LogQuestion(expertName, question string)
	LogResponse(expertName, response string)
}
