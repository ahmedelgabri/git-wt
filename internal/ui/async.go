package ui

// AsyncPhase describes the lifecycle of an async terminal UI.
type AsyncPhase int

const (
	AsyncLoading AsyncPhase = iota
	AsyncPartial
	AsyncReady
	AsyncError
	AsyncCanceled
)

// Done reports whether the phase is terminal.
func (p AsyncPhase) Done() bool {
	switch p {
	case AsyncReady, AsyncError, AsyncCanceled:
		return true
	default:
		return false
	}
}

// Active reports whether background work may still be running.
func (p AsyncPhase) Active() bool {
	return !p.Done()
}
