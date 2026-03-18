package httpz

import "net/http"

type responseRecorder struct {
	bytes  int
	status int
	http.ResponseWriter
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		status:         http.StatusOK,
		ResponseWriter: w,
	}
}

func (rr *responseRecorder) WriteHeader(status int) {
	rr.status = status
	rr.ResponseWriter.WriteHeader(status)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	n, err := rr.ResponseWriter.Write(b)
	rr.bytes += n
	return n, err
}
