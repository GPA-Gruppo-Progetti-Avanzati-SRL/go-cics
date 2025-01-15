package cicsservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarshal(t *testing.T) {
	type Test struct {
		Filed3 string `mainframe:"start=16,length=5" validate:"required"`
		Field2 int    `mainframe:"start=11,length=5" validate:"required"`
		Field1 int    `mainframe:"start=1,length=5" validate:"required"`
	}

	st := Test{
		Field1: 1,
		Filed3: "test",
		Field2: 1000,
	}

	value, err := Marshal(st)

	if assert.NotNil(t, value) {
		assert.Equal(t, value, []byte("00001\x00\x00\x00\x00\x0001000test "))
	}

	test2 := Test{}

	err = Unmarshal(value, &test2)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, st, test2)

}
