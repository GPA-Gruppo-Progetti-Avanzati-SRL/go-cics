package cics

import (
	"fmt"
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

func BuildHeaderV2(RequestInfo *RequestInfo, config *RoutineConfig) *HeaderV2 {
	conversion := "N"
	log_level := "N"

	header := &HeaderV2{
		ABIBanca:           "07601",
		ProgramName:        config.ProgramName,
		Conversion:         conversion,
		FlagDebug:          "",
		LogLevel:           log_level,
		ErrorDescription:   "",
		RequestIdClient:    RequestInfo.RequestId,
		CorrelationIdPoste: RequestInfo.TrackId,
		RequestIdLegacy:    RequestInfo.RequestId,
		TransId:            config.TransId,
	}
	return header

}

func BuildHeaderV3(requestInfo *RequestInfo, config *RoutineConfig) *HeaderV3 {

	log_level := "N"
	idem_potence := "N"
	requestInfo.TransactionSequence++
	subRequestId := fmt.Sprintf("%d%02d-%s", requestInfo.OrchestrationSequence, requestInfo.TransactionSequence, config.ProgramName)

	header := &HeaderV3{
		Version:          "VER1.0",
		ProgramName:      config.ProgramName,
		TransId:          config.TransId,
		SystemId:         requestInfo.SystemId,
		Canale:           requestInfo.Canale,
		Conversion:       CONVERSION,
		FlagDebug:        DEBUG,
		LogLevel:         log_level,
		RollBack:         "",
		ReturnCode:       "",
		ErrorDescription: "",
		FlagCheckIdem:    idem_potence,
		CicsTargetPool:   "",
		TrackingId:       requestInfo.TrackId,
		RequestId:        requestInfo.RequestId,
		SubRequestId:     subRequestId,
	}

	return header
}
