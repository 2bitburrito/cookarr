package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/hpalmer/cookarr/internal/store"
)

// marshalLayout re-encodes a decoded layout body back to compact JSON for
// storage. The server treats Dockview layouts as opaque blobs; it only needs
// well-formed JSON, not a known shape.
func marshalLayout(w http.ResponseWriter, layout map[string]any) (string, error) {
	if layout == nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "layout is required")
		return "", errEmptyLayout
	}
	raw, err := json.Marshal(layout)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid", "layout must be valid JSON")
		return "", err
	}
	return string(raw), nil
}

var errEmptyLayout = errNotJSON("layout is required")

type errNotJSON string

func (e errNotJSON) Error() string { return string(e) }

// writeRawLayout writes a stored layout blob back out, tagged with where it
// was resolved from (override vs. saved preset) so the frontend can show
// that in the UI if it wants to.
func writeRawLayout(w http.ResponseWriter, source, raw string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// raw is already valid JSON (the server only ever stores what it
	// validated on the way in), so splice it in without re-decoding.
	_, _ = w.Write([]byte(`{"source":"` + source + `","layout":` + raw + `}`))
}

func jsonMarshalString(s string) (string, error) {
	raw, err := json.Marshal(s)
	return string(raw), err
}

func jsonUnmarshalString(raw string, out *string) error {
	return json.Unmarshal([]byte(raw), out)
}

func presetByName(ctx context.Context, s store.Store, name, deviceClass string) (string, error) {
	preset, err := s.GetLayoutPreset(ctx, name, deviceClass)
	if err != nil {
		return "", err
	}
	return preset.Layout, nil
}
