package ib

// Errors specific to the IB provider.
var (
	ErrClientClosed = &ClientClosedError{}
)

// ClientClosedError is returned when an operation is attempted on a closed client.
type ClientClosedError struct{}

func (e *ClientClosedError) Error() string {
	return "IB client is closed"
}
