package httpz

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func WriteJSON(w http.ResponseWriter, status int, data Envelope, headers http.Header) error {
	encodedData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	for key, values := range headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_, _ = w.Write(encodedData)
	return nil
}

func ReadJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	err := decoder.Decode(destination)
	if err != nil {
		if _, ok := errors.AsType[*json.InvalidUnmarshalError](err); ok {
			panic(err)
		}

		if errors.Is(err, io.EOF) {
			return errors.New("body must not be empty")
		}

		if errors.Is(err, io.ErrUnexpectedEOF) {
			return errors.New("body contains badly-formed JSON")
		}

		if jsonErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return fmt.Errorf("body must not be larger than %d bytes", jsonErr.Limit)
		}

		if jsonErr, ok := errors.AsType[*json.SyntaxError](err); ok {
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", jsonErr.Offset)
		}

		if jsonErr, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
			return fmt.Errorf("body contains unknown key %s", jsonErr)
		}

		if jsonErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
			if jsonErr.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", jsonErr.Field)
			}

			return fmt.Errorf("body contains incorrect JSON type (at character %d)", jsonErr.Offset)
		}

		return err
	}

	err = decoder.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}
