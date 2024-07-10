package cics

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"reflect"
	"strconv"
	"strings"
)

var validate = *validator.New()

type MainframeTag struct {
	length int
	start  int
}

func KeyValueParser(s string) (map[string]string, error) {
	splitted := strings.Split(s, ",")
	keyValueMap := make(map[string]string)

	for _, keyValues := range splitted {
		t := strings.Split(keyValues, "=")
		keyValueMap[t[0]] = t[1]
	}
	return keyValueMap, nil
}

func Marshal(v interface{}) ([]byte, error) {
	value := reflect.ValueOf(v)

	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}

	err := validate.Struct(value)

	if err != nil {
		return nil, err
	}
	t := value.Type()

	builder := BufferWritePosition{}
	maxLength := 0
	lastByte := 0
	if value.Kind() == reflect.Struct {

		for i := 0; i < value.NumField(); i++ {
			valueTag := t.Field(i).Tag.Get("mainframe")
			if len(valueTag) == 0 {
				continue
			}
			fieldValue := value.Field(i)

			mainframeTag, err := parseMainFrameTag(valueTag)

			if err != nil {
				return nil, err
			}

			maxLength += mainframeTag.length
			currentLastByte := mainframeTag.start + mainframeTag.length
			if lastByte < (currentLastByte) {
				lastByte = currentLastByte
			}

			switch fieldValue.Kind() {
			case reflect.Int:
				// "%06d"
				builder.WriteStringPosition(fmt.Sprintf(fmt.Sprintf("%%0%dd", mainframeTag.length), fieldValue.Int()), mainframeTag.start)
			case reflect.String:
				s := fieldValue.String()
				if mainframeTag.length < len(s) {
					return nil, fmt.Errorf("can't marshal because field %s max length is %d and string lengh is %d", fieldValue.Type().Name(), mainframeTag.length, len(s))
				}
				builder.WriteStringPosition(fmt.Sprintf(fmt.Sprintf("%%-%ds", mainframeTag.length), s), mainframeTag.start)
			}

		}

	} else {
		return nil, errors.New("value is not a struct")
	}

	return builder.Buf[:lastByte], nil
}

func Unmarshal(data []byte, v any) error {
	value := reflect.ValueOf(v).Elem()

	t := value.Type()

	for i := 0; i < value.NumField(); i++ {
		valueTag := t.Field(i).Tag.Get("mainframe")
		if len(valueTag) == 0 {
			continue
		}
		fieldValue := value.Field(i)

		mainframeTag, err := parseMainFrameTag(valueTag)
		if err != nil {
			return err
		}
		if len(data) < mainframeTag.start+mainframeTag.length {
			return errors.New(fmt.Sprintf("Invalid data can not parse field %s", t.Field(i).Name))
		}
		switch fieldValue.Kind() {
		case reflect.Int:
			s, err := strconv.Atoi(string(bytes.Trim(data[mainframeTag.start:mainframeTag.start+mainframeTag.length], "\x00")))
			if err != nil {
				return err
			}
			fieldValue.SetInt(int64(s))
		case reflect.String:
			fieldValue.SetString(strings.Trim(string(bytes.Trim(data[mainframeTag.start:mainframeTag.start+mainframeTag.length], "\x00")), " "))
		}

	}

	err := validate.Struct(v)

	if err != nil {
		return err
	}

	return nil
}

func parseMainFrameTag(tag string) (MainframeTag, error) {
	mainframeTag := new(MainframeTag)

	valueTagParsed, err := KeyValueParser(tag)
	if err != nil {
		return *mainframeTag, err
	}
	positionWrite := 0
	if val, ok := valueTagParsed["start"]; ok {
		positionWrite, err = strconv.Atoi(val)
		if err != nil {
			return *mainframeTag, err
		}
		positionWrite--

	} else {
		return *mainframeTag, errors.New("mainframe tag defined but not start")
	}
	mainframeTag.start = positionWrite

	lenghtPadding, err := strconv.Atoi(valueTagParsed["length"])
	if err != nil {
		return *mainframeTag, err
	}
	mainframeTag.length = lenghtPadding

	return *mainframeTag, nil

}
