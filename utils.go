package cicsservice

import (
	"strings"
	"unicode"
)

const DEBUG = "N"
const CONVERSION = "N"

func (header *HeaderV3) IsError() bool {

	if header.ReturnCode == "000" || header.ReturnCode == "00000" {
		return false
	}
	return true
}

func (header *HeaderV2) IsError() bool {

	if header.CicsResponseCode == "000" || header.CicsResponse2Code == "00000" {
		return false
	}
	return true
}

type Header interface {
	IsError() bool
}

type Container struct{}

type RequestInfo struct {
	RequestId             string
	TrackId               string
	SystemId              string
	Canale                string
	OrchestrationSequence int
	TransactionSequence   int
}

func ClearString(s string) string {
	s1 := strings.TrimSpace(s)
	s1 = strings.Map(func(r rune) rune {
		if unicode.IsGraphic(r) {
			return r
		}
		return -1
	}, s1)
	return s1
}
