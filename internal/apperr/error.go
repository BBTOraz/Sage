package apperr

type ErrorType string

const (
	FileNotFound                ErrorType = "file_not_found"
	PolicyDenied                ErrorType = "policy_denied"
	EmptyPath                   ErrorType = "empty_path"
	InvalidToolArgument         ErrorType = "invalid_tool_argument"
	PolicyErrorInvalidPath      ErrorType = "invalid_path"
	PolicyErrorOutsideRoot      ErrorType = "outside_root"
	PolicyErrorPermissionDenied ErrorType = "permission_denied"
	PolicyErrorExecutionDenied  ErrorType = "execution_denied"
	UnknownError                ErrorType = "unknown_error"
	InvalidPath                 ErrorType = "invalid_path"
	InvalidToolCall             ErrorType = "invalid_tool_call"
)

type AppError interface {
	error
	Type() ErrorType
	Hint() string
}
