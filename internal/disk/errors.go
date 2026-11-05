package disk

import "fmt"

type Error string

func (e Error) Error() string { return string(e) }

func errf(format string, args ...any) error {
	return Error(fmt.Sprintf(format, args...))
}
