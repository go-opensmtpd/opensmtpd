package opensmtpd

import "strings"

func ExampleFilter() {
	// Build our filter
	filter := &Filter{
		HELO: func(session *Session, helo string) error {
			if helo == "test" {
				return session.Reject(FilterOK, 0)
			}
			return session.Accept()
		},
	}

	// Add another hook
	filter.MAIL = func(session *Session, user, domain string) error {
		if strings.ToLower(domain) == "example.org" {
			return session.Reject(FilterOK, 0)
		}
		return session.Accept()
	}

	// Register our filter with smtpd. This step is optional and will
	// be performed by Serve() if omitted.
	if err := filter.Register(); err != nil {
		panic(err)
	}

	// And keep serving until smtpd stops
	filter.Serve()
}
