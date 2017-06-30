package opensmtpd

import "strings"

func ExampleFilter() {
	// Build our filter
	filter := &Filter{
		HELO: func(session *Session, helo string) error {
			if helo == "test" {
				return session.Reject()
			}
			return session.Accept()
		},
	}

	// Add another hook
	filter.OnMAIL(func(session *Session, user, domain string) error {
		if strings.ToLower(domain) == "example.org" {
			return session.Reject()
		}
		return session.Accept()
	})

	// Register our filter with smtpd
	if err := filter.Register(); err != nil {
		panic(err)
	}

	// And keep serving until smtpd stops
	filter.Serve()
}
