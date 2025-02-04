package cicsservice

/*
#include  <ctgclient_eci.h>
#include  <string.h>
#include <stdlib.h>
#include <stdio.h>
*/
import "C"
import (
	"fmt"
	"github.com/rs/zerolog/log"
	"golang.org/x/text/encoding/charmap"
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

func convertToAscii(data []byte) []byte {
	decoder := charmap.CodePage037.NewDecoder()
	output, errorD := decoder.Bytes(data)
	if errorD != nil {
		log.Error().Err(errorD).Msgf("Error %s ", errorD.Error())
	}
	return output
}

func strCopy8(dest *[8]C.char, src string) {
	for i, c := range src {
		dest[i] = C.char(c)
	}
}

func strCopy4(dest *[4]C.char, src string) {
	for i, c := range src {
		dest[i] = C.char(c)
	}
}

func displayRc(ctgRc C.int) *TransactionError {
	ptr := C.malloc(C.sizeof_char * (C.CTG_MAX_RCSTRING + 1))
	C.memset(ptr, C.sizeof_char*(C.CTG_MAX_RCSTRING+1), 0)
	C.CTG_getRcString(ctgRc, (*C.char)(ptr))
	defer C.free(ptr)
	returnString := C.GoBytes(ptr, C.sizeof_char*(C.CTG_MAX_RCSTRING+1))
	log.Trace().Msgf("ErrorCode : %v ,ErrorMessage %s", ctgRc, ClearString(string(returnString)))
	return &TransactionError{
		ErrorCode: fmt.Sprintf("%v", ctgRc), ErrorMessage: ClearString(string(returnString)),
	}
}
