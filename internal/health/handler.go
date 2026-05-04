package health

import "net/http"

func Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
