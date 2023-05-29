package status

import (
	"fmt"

	"google.golang.org/grpc/codes"
)

type Status struct {
	code   codes.Code
	reason string
}

// FromError returns a Status representation of err.
//
//   - If err was produced by this package or implements the method `GRPCStatus()
//     *Status`, the appropriate Status is returned.
//
//   - If err is nil, a Status is returned with codes.OK and no message.
//
//   - Otherwise, err is an error not compatible with this package.  In this
//     case, a Status is returned with codes.Unknown and err's Error() message,
//     and ok is false.
func FromError(err error) (s *Status, ok bool) {
	if err == nil {
		return nil, true
	}

	if se, ok := err.(*Status); ok {
		return se, true
	}

	return New(codes.Internal, err.Error()), false
}

// New returns a Status representing c and msg.
func New(c codes.Code, msg string) *Status {
	return &Status{
		code:   c,
		reason: msg,
	}
}

// Newf returns New(c, fmt.Sprintf(format, a...)).
func Newf(c codes.Code, format string, a ...interface{}) *Status {
	return New(c, fmt.Sprintf(format, a...))
}

// Error returns an error representing c and msg.  If c is OK, returns nil.
func Error(c codes.Code, msg string) error {
	return New(c, msg)
}

// Errorf returns Error(c, fmt.Sprintf(format, a...)).
func Errorf(c codes.Code, format string, a ...interface{}) error {
	return Error(c, fmt.Sprintf(format, a...))
}

func (e *Status) Code() codes.Code {
	return e.code
}

func (e *Status) Reason() string {
	return e.reason
}

func (e *Status) Error() string {
	return fmt.Sprintf("code = %s, reason = %s", e.Code().String(), e.Reason())
}

func (e *Status) Is(target error) bool {
	tse, ok := target.(*Status)
	if !ok {
		return false
	}
	return e.Code() == tse.Code() && e.Reason() == tse.Reason()
}
