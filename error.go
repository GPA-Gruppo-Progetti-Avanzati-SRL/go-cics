package cicsservice

import (
	"fmt"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
)

type TransactionError struct {
	ErrorCode     string `json:"ErrorCode" mainframe:"start=1,length=11"`
	ErrorMessage  string `json:"ErrorMessage"  mainframe:"start=12,length=100"`
	ErrorMessage2 string `json:"ErrorMessage2" mainframe:"start=112,length=100"`
}

func (t *TransactionError) Error() string {
	return fmt.Sprintf("Error  %s - %s%s", t.ErrorCode, t.ErrorMessage, t.ErrorMessage2)
}

func TechnicalErrorFromTransaction(rn string, t *TransactionError) *core.ApplicationError {
	return &core.ApplicationError{
		StatusCode: 500,
		Ambit:      rn,
		Code:       t.ErrorCode,
		Message:    fmt.Sprintf("%s %s", t.ErrorMessage, t.ErrorMessage2),
	}
}

func BusinessErroFromTransaction(rn string, t *TransactionError) *core.ApplicationError {
	return &core.ApplicationError{
		StatusCode: 400,
		Ambit:      rn,
		Code:       t.ErrorCode,
		Message:    fmt.Sprintf("%s %s", t.ErrorMessage, t.ErrorMessage2),
	}
}
