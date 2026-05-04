package httpx

import "net/http"

func WriteBasicAuthChallenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Gitta"`)
	http.Error(w, "authentication required", http.StatusUnauthorized)
}

func WriteNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("expires", "Fri, 01 Jan 1980 00:00:00 GMT")
	w.Header().Set("pragma", "no-cache")
	w.Header().Set("cache-control", "no-cache, max-age=0, must-revalidate")
}
