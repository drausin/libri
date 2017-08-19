package server

import (
	"testing"

	"fmt"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewDevLogger(t *testing.T) {
	l := NewDevLogger(zap.DebugLevel)
	assert.NotNil(t, l)
}

func TestNewDevInfoLogger(t *testing.T) {
	l := NewDevInfoLogger()
	assert.NotNil(t, l)
}

func TestNewProdLogger(t *testing.T) {
	l := NewProdLogger(zap.InfoLevel)
	assert.NotNil(t, l)
}

func TestToErrArray(t *testing.T) {
	nErrs := 3
	errMap := make(map[string]error)
	for i := 0; i < nErrs; i++ {
		err := fmt.Errorf("error %d", i)
		errMap[string(i)] = err
	}
	assert.Equal(t, nErrs, len(ToErrArray(errMap)))
}

func TestErrArray_MarshalLogArray(t *testing.T) {
	errs := ErrArray{errors.New("error 1"), errors.New("error 2"), errors.New("error 3")}
	oe := zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()).(zapcore.ArrayEncoder)
	err := errs.MarshalLogArray(oe)
	assert.Nil(t, err)
}
