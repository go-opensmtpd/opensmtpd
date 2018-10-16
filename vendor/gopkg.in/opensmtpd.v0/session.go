package opensmtpd

type Session struct {
	ID uint64

	filter *Filter
	qtype  int
	qid    uint64
}

func NewSession(f *Filter, id uint64) *Session {
	return &Session{
		ID:     id,
		filter: f,
	}
}

func (s *Session) Accept() error {
	return s.filter.respond(s, FilterOK, 0, "")
}

func (s *Session) AcceptCode(code int, line string) error {
	return s.filter.respond(s, FilterOK, code, line)
}

func (s *Session) Reject(status, code int) error {
	if status == FilterOK {
		status = FilterFail
	}

	return s.filter.respond(s, status, code, "")
}

func (s *Session) RejectCode(status, code int, line string) error {
	if status == FilterOK {
		status = FilterFail
	}

	return s.filter.respond(s, status, code, line)
}
