// 09
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func encode[T any](w http.ResponseWriter, _ *http.Request, status int, v T) error {
	buf, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(buf)
	return err
}

func decode[T any](r *http.Request) (T, error) {
	var v T
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return v, errors.New("request body must contain a single JSON object")
	}
	return v, nil
}
