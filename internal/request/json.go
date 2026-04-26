package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	return decodeJSON(w, r, dst, false)
}

func DecodeJSONStrict(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	return decodeJSON(w, r, dst, true)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}, disallowUnknownFields bool) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)

	dec := json.NewDecoder(r.Body)

	if disallowUnknownFields {
		dec.DisallowUnknownFields()
	}

	err := dec.Decode(dst)
	if err != nil {
		return decodeJSONError(err)
	}

	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

type decodeErrorHandler func(error) (error, bool)

func decodeJSONError(err error) error {
	for _, handler := range decodeErrorHandlers {
		if message, handled := handler(err); handled {
			return message
		}
	}
	return err
}

var decodeErrorHandlers = []decodeErrorHandler{
	emptyJSONBodyError,
	jsonSyntaxError,
	unexpectedJSONEOFError,
	jsonTypeError,
	unknownJSONFieldError,
	maxJSONBytesError,
	invalidJSONUnmarshalError,
}

func emptyJSONBodyError(err error) (error, bool) {
	if errors.Is(err, io.EOF) {
		return errors.New("body must not be empty"), true
	}
	return nil, false
}

func jsonSyntaxError(err error) (error, bool) {
	var syntaxError *json.SyntaxError
	if errors.As(err, &syntaxError) {
		return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset), true
	}
	return nil, false
}

func unexpectedJSONEOFError(err error) (error, bool) {
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return errors.New("body contains badly-formed JSON"), true
	}
	return nil, false
}

func jsonTypeError(err error) (error, bool) {
	var unmarshalTypeError *json.UnmarshalTypeError
	if !errors.As(err, &unmarshalTypeError) {
		return nil, false
	}
	if unmarshalTypeError.Field != "" {
		return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field), true
	}
	return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset), true
}

func unknownJSONFieldError(err error) (error, bool) {
	if !strings.HasPrefix(err.Error(), "json: unknown field ") {
		return nil, false
	}
	fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
	return fmt.Errorf("body contains unknown key %s", fieldName), true
}

func maxJSONBytesError(err error) (error, bool) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit), true
	}
	return nil, false
}

func invalidJSONUnmarshalError(err error) (error, bool) {
	var invalidUnmarshalError *json.InvalidUnmarshalError
	if errors.As(err, &invalidUnmarshalError) {
		panic(err)
	}
	return nil, false
}
