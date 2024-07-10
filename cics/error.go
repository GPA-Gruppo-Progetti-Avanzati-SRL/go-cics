package cics

import "fmt"

type TransactionError struct {
	Status        int    `json:"-"`
	ErrorCode     string `json:"ErrorCode" mainframe:"start=1,length=11"`
	ErrorMessage  string `json:"ErrorMessage"  mainframe:"start=12,length=100"`
	ErrorMessage2 string `json:"ErrorMessage2" mainframe:"start=112,length=100"`
}

func (t *TransactionError) Error() string {
	return fmt.Sprintf("Error  %s - %s%s", t.ErrorCode, t.ErrorMessage, t.ErrorMessage2)
}

func (e *TransactionError) GetStatus() int {
	return e.Status
}
func TransactionErrorFromError(err error) *TransactionError {
	return &TransactionError{
		Status:       500,
		ErrorCode:    "99999",
		ErrorMessage: err.Error(),
	}
}
