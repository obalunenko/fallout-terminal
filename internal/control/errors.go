package control

import "errors"

var (
	// ErrCommandExecutionStale identifies a decision that no longer matches
	// the current pending command execution.
	ErrCommandExecutionStale = errors.New("command execution decision is stale")
	// ErrCommandExecutionPersistence identifies a failure to prove and install
	// the durable result of a state-changing command.
	ErrCommandExecutionPersistence = errors.New("command execution could not be persisted")
)

type commandExecutionFailure struct {
	message  string
	identity error
}

func (failure commandExecutionFailure) Error() string {
	return failure.message
}

func (failure commandExecutionFailure) Unwrap() error {
	return failure.identity
}

func commandExecutionPersistenceFailure(message string) error {
	if message == ErrCommandExecutionPersistence.Error() {
		return ErrCommandExecutionPersistence
	}
	return commandExecutionFailure{message: message, identity: ErrCommandExecutionPersistence}
}
