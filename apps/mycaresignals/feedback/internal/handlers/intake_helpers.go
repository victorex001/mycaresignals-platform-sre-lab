package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxIntakeBodyBytes = 96 * 1024

var errInvalidIntakeBody = errors.New("invalid intake payload")

func decodeIntakeBody(r *http.Request, destination any) ([]byte, error) {
	limited := io.LimitReader(r.Body, maxIntakeBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil || len(body) == 0 || len(body) > maxIntakeBodyBytes {
		return nil, errInvalidIntakeBody
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(destination); err != nil {
		return nil, errInvalidIntakeBody
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errInvalidIntakeBody
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return nil, errInvalidIntakeBody
	}
	delete(raw, "context_token")
	return json.Marshal(raw)
}
