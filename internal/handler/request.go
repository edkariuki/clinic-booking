package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

const maxBodyBytes = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: request body must be valid JSON with only the documented fields", domain.ErrInvalidInput)
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("%w: request body must contain a single JSON object", domain.ErrInvalidInput)
	}
	return nil
}
