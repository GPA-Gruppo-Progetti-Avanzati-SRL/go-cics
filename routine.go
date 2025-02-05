package cicsservice

/*
#include  <ctgclient_eci.h>
#include  <string.h>
#include <stdlib.h>
#include <stdio.h>
*/
import "C"

import (
	"context"
	"fmt"
	"github.com/GPA-Gruppo-Progetti-Avanzati-SRL/go-core-app"
	"github.com/ianlopshire/go-fixedwidth"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"time"
	"unsafe"
)

type Routine[I, O any] struct {
	Name                              string
	Config                            *RoutineConfig
	RequestInfo                       *RequestInfo
	Metrics                           *Metrics
	GenerateInputContainerFromInput   func(*I) (map[string][]byte, *core.ApplicationError)
	GenerateOutputFromOutputContainer func(map[string][]byte) (*O, *core.ApplicationError)
}

const HEADER = "HEADER"
const INPUT = "INPUT"
const ERRORE = "ERRORE"
const OUTPUT = "OUTPUT"
const CICSLIBERRORCODE = "99999"

func (cr *Routine[I, O]) TransactParsed() *TransactionError {
	return nil
}

func (cr *Routine[I, O]) TransactV3(ctx context.Context, connection *Connection, input *I) (*O, *core.ApplicationError) {

	return cr.transact(ctx, connection, input, cr.BuildHeaderV3, cr.checkOutputContainerV3)
}
func (cr *Routine[I, O]) TransactV2(ctx context.Context, connection *Connection, input *I) (*O, *core.ApplicationError) {

	return cr.transact(ctx, connection, input, cr.BuildHeaderV2, cr.checkOutputContainerV2)

}

func (cr *Routine[I, O]) BuildHeaderV2() Header {
	conversion := "N"
	log_level := "N"

	header := &HeaderV2{
		ABIBanca:           "07601",
		ProgramName:        cr.Config.ProgramName,
		Conversion:         conversion,
		FlagDebug:          "",
		LogLevel:           log_level,
		ErrorDescription:   "",
		RequestIdClient:    cr.RequestInfo.RequestId,
		CorrelationIdPoste: cr.RequestInfo.TrackId,
		RequestIdLegacy:    cr.RequestInfo.RequestId,
		TransId:            cr.Config.TransId,
	}
	return header

}

func (cr *Routine[I, O]) BuildHeaderV3() Header {

	requestInfo := cr.RequestInfo
	config := cr.Config

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

func (cr *Routine[I, O]) transact(ctx context.Context, connection *Connection, input *I, hf func() Header, ppf func(map[string][]byte) *core.ApplicationError) (*O, *core.ApplicationError) {

	ic, ierr := cr.GenerateInputContainerFromInput(input)
	if ierr != nil {
		return nil, ierr
	}
	header, herr := fixedwidth.Marshal(hf())
	if herr != nil {
		return nil, core.TechnicalErrorWithError(herr)
	}
	ic[HEADER] = header

	for key, element := range ic {
		log.Trace().Msgf("INPUTCONTAINER %s-%s*EOC*", key, element)
	}

	var token C.ECI_ChannelToken_t

	pChannel := C.CString(cr.Config.ChannelName)
	defer C.free(unsafe.Pointer(pChannel))

	C.ECI_createChannel(pChannel, &token)
	defer func() {
		EciChannel <- &token
	}()

	errinput := cr.buildContainer(token, ic)
	if errinput != nil {
		return nil, core.TechnicalErrorWithError(errinput)
	}
	var eciParms C.CTG_ECI_PARMS = cr.getEciParams(token, connection)
	if connection.Config.UserName != "" && connection.Config.Password != "" {
		pUserName := C.CString(connection.Config.UserName)
		pPassword := C.CString(connection.Config.Password)
		defer C.free(unsafe.Pointer(pUserName))
		defer C.free(unsafe.Pointer(pPassword))
		cr.setAuth(&eciParms, pUserName, pPassword)
	}

	if connection.ConnectionToken == nil {
		return nil, TechnicalErrorFromTransaction(cr.Name, &TransactionError{ErrorCode: CICSLIBERRORCODE,
			ErrorMessage: "No Cics connection Present"})
	}
	start := time.Now()
	defer func() {
		cr.Metrics.TransactionDuration.Record(ctx, time.Since(start).Milliseconds(), metric.WithAttributes(attribute.String("program", cr.Config.ProgramName)))
	}()

	ctx, cancel := context.WithTimeout(ctx, connection.Config.Timeout)
	defer cancel()
	var ctoken C.CTG_ConnToken_t = *connection.ConnectionToken
	var ctgRc C.int
	processDone := make(chan bool)
	log.Debug().Msgf("Execute Channel Transaction with timeout %d", connection.Config.Timeout)
	go func(ctgRc C.int) {
		ctgRc = C.CTG_ECI_Execute_Channel(ctoken, &eciParms)
		processDone <- true
	}(ctgRc)
	select {
	case <-ctx.Done():
		ctgRc = C.ECI_ERR_SYSTEM_ERROR
		log.Warn().Msg("Timed Out")
		break
	case <-processDone:
		break
	}

	if ctgRc != C.ECI_NO_ERROR {
		log.Trace().Msg("Ho errore")
		conntoken := connection.ConnectionToken
		TokenChannel <- conntoken
		connection.ConnectionToken = nil
		return nil, TechnicalErrorFromTransaction(cr.Config.ProgramName, displayRc(ctgRc))
	}

	oc, err := cr.getOutputContainer(token)
	if err != nil {
		return nil, TechnicalErrorFromTransaction(cr.Config.ProgramName, err)
	}

	if errCO := ppf(oc); errCO != nil {
		log.Trace().Msgf("Ho container error %s", errCO.Error())
		return nil, errCO
	}
	log.Trace().Msgf("Genero Output")
	return cr.GenerateOutputFromOutputContainer(oc)

}

func (cr *Routine[I, O]) getOutputContainer(token C.ECI_ChannelToken_t) (map[string][]byte, *TransactionError) {
	oc := make(map[string][]byte)
	var contInfo C.ECI_CONTAINER_INFO
	ctgRc := C.ECI_getFirstContainer(token, &contInfo)
	for ctgRc == C.ECI_NO_ERROR {
		offset := C.size_t(0)

		dataBuff := C.malloc(C.sizeof_char * (contInfo.dataLength + 1))
		C.memset(dataBuff, C.sizeof_char*(C.CTG_MAX_RCSTRING+1), 0)
		bytesRead := C.size_t(0)
		containerNameSlice := C.GoBytes(unsafe.Pointer(&contInfo.name), C.int(16))
		containerName := ClearString(string(containerNameSlice))

		input := C.CString(containerName)
		for (offset < C.ulong(contInfo.dataLength)) && (ctgRc == C.ECI_NO_ERROR) {
			ctgRc = C.ECI_getContainerData(token, input, dataBuff, C.ulong(contInfo.dataLength), offset, &bytesRead)
			offset += bytesRead

		}
		C.free(unsafe.Pointer(input))
		containerContentSlice := C.GoBytes(dataBuff, C.int(contInfo.dataLength))
		if contInfo.ccsid == 37 {
			containerContentSlice = convertToAscii(containerContentSlice)
		}

		oc[containerName] = containerContentSlice
		log.Trace().Msgf("OUTPUTCONTAINER , %s-%s*EOC*", containerName, containerContentSlice)
		C.free(dataBuff)
		ctgRc = C.ECI_getNextContainer(token, &contInfo)
	}
	return oc, nil
}

func (cr *Routine[I, O]) checkOutputContainerV3(oc map[string][]byte) *core.ApplicationError {

	headerm, applicationError := getHeaderContainer(cr.Name, oc)
	if applicationError != nil {
		return applicationError
	}

	header := &HeaderV3{}
	err := fixedwidth.Unmarshal(headerm, header)
	if err != nil {
		return TechnicalErrorFromTransaction(cr.Config.ProgramName, &TransactionError{
			ErrorCode:    CICSLIBERRORCODE,
			ErrorMessage: "Unable to unmarshal header V3",
		})
	}

	log.Trace().Msgf("Return Header V3 : %v\n", header)

	if header.ReturnCode == "000" || header.ReturnCode == "00000" {
		return nil
	}
	return BusinessErroFromTransaction(cr.Config.ProgramName, getErrorContainer(oc[ERRORE]))

}

func (cr *Routine[I, O]) checkOutputContainerV2(oc map[string][]byte) *core.ApplicationError {

	headerm, applicationError := getHeaderContainer(cr.Config.ProgramName, oc)
	if applicationError != nil {
		return applicationError
	}

	header := &HeaderV2{}
	err := fixedwidth.Unmarshal(headerm, header)
	if err != nil {
		return TechnicalErrorFromTransaction(cr.Name, &TransactionError{
			ErrorCode:    CICSLIBERRORCODE,
			ErrorMessage: "Unable to unmarshal header V2 ",
		})
	}

	log.Trace().Msgf("Return Header V2 : %v\n", header)

	if header.CicsResponseCode == "000" || header.CicsResponse2Code == "00000" {
		return nil
	}
	return BusinessErroFromTransaction(cr.Name, getErrorContainer(oc[ERRORE]))

}

func getHeaderContainer(name string, oc map[string][]byte) ([]byte, *core.ApplicationError) {
	if oc == nil {
		return nil, TechnicalErrorFromTransaction(name, &TransactionError{
			ErrorCode:    CICSLIBERRORCODE,
			ErrorMessage: "no container present",
		})
	}
	headerm, ok := oc[HEADER]
	if !ok {
		return nil, TechnicalErrorFromTransaction(name, &TransactionError{
			ErrorCode:    CICSLIBERRORCODE,
			ErrorMessage: "no container header present",
		})
	}
	return headerm, nil
}

func getErrorContainer(s []byte) *TransactionError {
	err := &TransactionError{}
	errUm := fixedwidth.Unmarshal(s, err)
	if errUm != nil {
		log.Warn().Msgf("Unable to unmarshal  Error : %s", string(s))
	}
	log.Trace().Msgf("Return Error : %v\n", err)
	return err
}

func (cr *Routine[I, O]) buildContainer(token C.ECI_ChannelToken_t, ic map[string][]byte) *core.ApplicationError {

	for key, element := range ic {
		pKey := C.CString(key)
		ctgRc := C.ECI_createContainer(token, pKey, C.ECI_CHAR, 0, unsafe.Pointer(&element[0]), C.ulong(len(element)))
		C.free(unsafe.Pointer(pKey))
		if ctgRc != C.ECI_NO_ERROR {
			return TechnicalErrorFromTransaction(cr.Name, &TransactionError{
				ErrorCode:    CICSLIBERRORCODE,
				ErrorMessage: "Errore set Container Input : " + key,
			})
		}

	}
	return nil
}

func (cr *Routine[I, O]) getEciParams(token C.ECI_ChannelToken_t, connection *Connection) C.CTG_ECI_PARMS {
	var eciParms C.CTG_ECI_PARMS
	/* ECI parameter block */
	programName := cr.Config.CicsGatewayName
	transId := cr.Config.TransId
	tpn := ""
	serverName := connection.Config.ServerName

	eciParms.eci_version = C.ECI_VERSION_2A /* ECI version 2A          */
	eciParms.eci_call_type = C.ECI_SYNC     /* Synchronous ECI call    */

	eciParms.eci_extend_mode = C.ECI_NO_EXTEND                               /* Non-extended call       */
	eciParms.eci_luw_token = C.ECI_LUW_NEW                                   /* Zero for a new LUW      */
	eciParms.eci_timeout = C.short(int(connection.Config.Timeout.Seconds())) /* Timeout in seconds      */

	programNameChar := [8]C.char{}
	serverNameChar := [8]C.char{}
	strCopy8(&programNameChar, programName)
	strCopy8(&serverNameChar, serverName)
	eciParms.eci_program_name = programNameChar
	eciParms.eci_system_name = serverNameChar
	transIdChar := [4]C.char{}
	tpnChar := [4]C.char{}
	strCopy4(&transIdChar, transId)
	strCopy4(&tpnChar, tpn)
	eciParms.eci_transid = transIdChar
	eciParms.eci_tpn = tpnChar

	eciParms.channel = token
	return eciParms
}
func (cr *Routine[I, O]) setAuth(eciParms *C.CTG_ECI_PARMS, user *C.char, passwd *C.char) {

	eciParms.eci_userid_ptr = user
	eciParms.eci_password_ptr = passwd

}
