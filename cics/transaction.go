package cics

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
	"time"
	"unsafe"

	"github.com/rs/zerolog/log"
	"golang.org/x/text/encoding/charmap"
)

type Routine struct {
	Config          *RoutineConfig
	Connection      *Connection
	InputContainer  map[string][]byte
	OutputContainer map[string][]byte
}

const HEADER = "HEADER"
const INPUT = "INPUT"
const ERRORE = "ERRORE"
const OUTPUT = "OUTPUT"
const CICSLIBERRORCODE = "99999"

func (cr *Routine) TransactParsed() *TransactionError {
	return nil
}

func (cr *Routine) TransactV3(ctx context.Context) *TransactionError {

	err := cr.Transact(ctx)
	if err != nil {
		return err
	}
	err = cr.checkOutputContainer()
	if err != nil {
		return err
	}
	return nil
}

func (cr *Routine) Transact(ctx context.Context) *TransactionError {
	for key, element := range cr.InputContainer {
		log.Trace().Msgf("INPUTCONTAINER %s-%s*EOC*", key, element)
	}

	var token C.ECI_ChannelToken_t

	pChannel := C.CString(cr.Config.ChannelName)
	defer C.free(unsafe.Pointer(pChannel))

	C.ECI_createChannel(pChannel, &token)
	defer func() {
		EciChannel <- &token
	}()

	errinput := cr.buildContainer(token)
	if errinput != nil {
		return errinput
	}
	var eciParms C.CTG_ECI_PARMS = cr.getEciParams(token)
	if cr.Connection.Config.UserName != "" && cr.Connection.Config.Password != "" {
		pUserName := C.CString(cr.Connection.Config.UserName)
		pPassword := C.CString(cr.Connection.Config.Password)
		defer C.free(unsafe.Pointer(pUserName))
		defer C.free(unsafe.Pointer(pPassword))
		cr.setAuth(&eciParms, pUserName, pPassword)
	}

	if cr.Connection.ConnectionToken == nil {
		return &TransactionError{ErrorCode: CICSLIBERRORCODE,
			ErrorMessage: "No Cics connection Present"}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(cr.Connection.Config.Timeout+1)*time.Second)
	defer cancel()
	var ctoken C.CTG_ConnToken_t = *cr.Connection.ConnectionToken
	var ctgRc C.int
	processDone := make(chan bool)
	log.Debug().Msgf("Execute Channel Transaction with timeout %d", cr.Connection.Config.Timeout+1)
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
		conntoken := cr.Connection.ConnectionToken
		TokenChannel <- conntoken
		cr.Connection.ConnectionToken = nil
		return displayRc(ctgRc)
	}

	err := cr.getOutputContainer(token)
	return err
}

func (cr *Routine) getOutputContainer(token C.ECI_ChannelToken_t) *TransactionError {
	cr.OutputContainer = make(map[string][]byte)
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

		cr.OutputContainer[containerName] = containerContentSlice
		log.Trace().Msgf("OUTPUTCONTAINER , %s-%s*EOC*", containerName, containerContentSlice)
		C.free(dataBuff)
		ctgRc = C.ECI_getNextContainer(token, &contInfo)
	}
	return nil
}

func (cr *Routine) checkOutputContainer() *TransactionError {

	if cr.OutputContainer == nil {
		return &TransactionError{
			ErrorCode:    CICSLIBERRORCODE,
			ErrorMessage: "no container present",
		}
	}
	if len(cr.OutputContainer[HEADER]) == 0 {
		return &TransactionError{
			ErrorCode:    CICSLIBERRORCODE,
			ErrorMessage: "no container header present",
		}
	}

	header := &HeaderV3{}
	err := Unmarshal(cr.OutputContainer[HEADER], header)
	if err != nil {
		return &TransactionError{
			ErrorCode:    CICSLIBERRORCODE,
			ErrorMessage: "Unable to unmarshal header",
		}
	}

	log.Trace().Msgf("Return Header : %v\n", header)

	if header.ReturnCode == "000" || header.ReturnCode == "00000" {
		return nil
	}
	return getErrorContainer(cr.OutputContainer[ERRORE])

}

func getErrorContainer(s []byte) *TransactionError {
	err := &TransactionError{}
	Unmarshal(s, err)
	return err
}

func (cr *Routine) buildContainer(token C.ECI_ChannelToken_t) *TransactionError {
	for key, element := range cr.InputContainer {
		pKey := C.CString(key)
		ctgRc := C.ECI_createContainer(token, pKey, C.ECI_CHAR, 0, unsafe.Pointer(&element[0]), C.ulong(len(element)))
		C.free(unsafe.Pointer(pKey))
		if ctgRc != C.ECI_NO_ERROR {
			return &TransactionError{
				ErrorCode:    CICSLIBERRORCODE,
				ErrorMessage: "Errore set Container Input : " + key,
			}
		}

	}
	return nil
}

func (cr *Routine) getEciParams(token C.ECI_ChannelToken_t) C.CTG_ECI_PARMS {
	var eciParms C.CTG_ECI_PARMS
	/* ECI parameter block */
	programName := cr.Config.CicsGatewayName
	transId := cr.Config.TransId
	tpn := ""
	serverName := cr.Connection.Config.ServerName

	eciParms.eci_version = C.ECI_VERSION_2A /* ECI version 2A          */
	eciParms.eci_call_type = C.ECI_SYNC     /* Synchronous ECI call    */

	eciParms.eci_extend_mode = C.ECI_NO_EXTEND                   /* Non-extended call       */
	eciParms.eci_luw_token = C.ECI_LUW_NEW                       /* Zero for a new LUW      */
	eciParms.eci_timeout = C.short(cr.Connection.Config.Timeout) /* Timeout in seconds      */

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
func (cr *Routine) setAuth(eciParms *C.CTG_ECI_PARMS, user *C.char, passwd *C.char) {

	eciParms.eci_userid_ptr = user
	eciParms.eci_password_ptr = passwd

}

func convertToAscii(data []byte) []byte {
	decoder := charmap.CodePage037.NewDecoder()
	output, error := decoder.Bytes(data)
	if error != nil {
		fmt.Println("Error ", error)
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
