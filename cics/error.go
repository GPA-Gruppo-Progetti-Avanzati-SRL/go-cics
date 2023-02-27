package cics

import "fmt"

type TransactionError struct {
	ErrorCode     string `mainframe:"start=1,length=11"`
	ErrorMessage  string `mainframe:"start=12,length=100"`
	ErrorMessage2 string `mainframe:"start=112,length=100"`
}

func (t *TransactionError) Error() string {
	return fmt.Sprintf("Error  %s - %s%s", t.ErrorCode, t.ErrorMessage, t.ErrorMessage2)
}
