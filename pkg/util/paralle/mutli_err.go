package paralle

import "strings"

type MultiError struct {
	errs []error
}

func (s *MultiError) Len() int {
	return len(s.errs)
}

func (s *MultiError) AddError(err error) {
	if s.errs == nil {
		s.errs = make([]error, 0, 5)
	}

	s.errs = append(s.errs, err)
}

func (s *MultiError) Error() string {
	if s.errs == nil || len(s.errs) == 0 {
		return ""
	}

	builder := &strings.Builder{}
	for i := range s.errs {
		builder.WriteRune('\n')
		builder.WriteString("<MultiError>: ")
		builder.WriteString(s.errs[i].Error())
	}

	return builder.String()
}

func (s *MultiError) ERR() error {
	if s.Len() != 0 {
		return s
	}
	return nil
}
