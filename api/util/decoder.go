package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang/gddo/httputil/header"
)

const maxBytesAllowed = 1048576

type MalformedRequest struct {
	status int
	msg    string
}

func (m *MalformedRequest) Error() string {
	return m.msg
}

func DecodeJSONBody(w http.ResponseWriter, r *http.Request, m interface{}) error {
	// if Content-Type header present, validate that it is application/json
	// use of `gddo/httputil/header` will ensure extraction even if the client includes additional charset
	// or boundary information in the header
	if r.Header.Get("Content-Type") != "" {
		value, _ := header.ParseValueAndParams(r.Header, "Content-Type")
		if value != "application/json" {
			msg := "Content-Type header is not application/json"
			return &MalformedRequest{status: http.StatusUnsupportedMediaType, msg: msg}
		}
	}

	// now, we enforce a maximum read of 1MB from the response body
	// a larger body will result in `Decode` returning an error in kind
	r.Body = http.MaxBytesReader(w, r.Body, maxBytesAllowed)

	// init decoder and check for invalid fields in json body
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&m)

	if err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalTypeErr *json.UnmarshalTypeError

		switch {
		// capture JSON syntax errs and extract loc of said err
		case errors.As(err, &syntaxErr):
			msg := fmt.Sprintf("Request body contains malformed JSON at postition %d", syntaxErr.Offset)
			return &MalformedRequest{status: http.StatusBadRequest, msg: msg}

		// under some circumstances, `Decode` may return a `io.ErrUnexpectedEOF` err for syntax errs in JSON
		// handle these; see https://github.com/golang/go/issues/25956
		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := "Request body contains malformed JSON"
			return &MalformedRequest{status: http.StatusBadRequest, msg: msg}

		// capture type errs e.g. int value being mapped to a str field
		case errors.As(err, &unmarshalTypeErr):
			Field, Offset := unmarshalTypeErr.Field, unmarshalTypeErr.Offset
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field at position %d", Field, Offset)
			return &MalformedRequest{status: http.StatusBadRequest, msg: msg}

		// catch errors resulting from unexpected fields in the request body
		// pending change into sentinel error; see https://github.com/golang/go/issues/29035
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return &MalformedRequest{status: http.StatusBadRequest, msg: msg}

		// if the request body is empty, we receive an `io.EOF` err from `Decode`
		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return &MalformedRequest{status: http.StatusBadRequest, msg: msg}

		// request body exceeds `maxBytesAllowed`
		// pending change into sentinel error; see https://github.com/golang/go/issues/30715
		case err.Error() == "http: request body too large":
			msg := "Request body must not exceed 1MB" // TODO size to const bytes -> readable
			return &MalformedRequest{status: http.StatusRequestEntityTooLarge, msg: msg}

		default:
			return err
		}
	}

	// invoke `Decode` again, using a pointer to an empty anon struct as the dest; if rq body only contained
	// a single JSON object, this will return `io.EOF` - thus we will know if we recv excess data in the rq body
	err = dec.Decode(&struct{}{})

	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		return &MalformedRequest{status: http.StatusBadRequest, msg: msg}
	}

	return nil
}
