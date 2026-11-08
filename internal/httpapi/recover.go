package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

type sessionKey struct{}

// setSessionID tells the recovery middleware which session a request belongs
// to, once it is known.
func setSessionID(ctx context.Context, id string) {
	if holder, ok := ctx.Value(sessionKey{}).(*string); ok {
		*holder = id
	}
}

// recoverPanics turns a panic in next into a 500 problem and an ERROR log with
// the stack. The panic value is not logged, as it may hold request data. If
// the response has already started, the connection is aborted instead.
func recoverPanics(logger *slog.Logger, p problems, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sessionID string
		r = r.WithContext(context.WithValue(r.Context(), sessionKey{}, &sessionID))
		tw := &trackingWriter{ResponseWriter: w}
		defer func() {
			value := recover()
			if value == nil {
				return
			}
			if value == http.ErrAbortHandler {
				panic(value)
			}
			log := logger
			if sessionID != "" {
				log = log.With("session_id", sessionID)
			}
			log.Error("panic in handler", "panic_type", fmt.Sprintf("%T", value), "stack", string(debug.Stack()))
			if tw.wroteHeader {
				panic(http.ErrAbortHandler)
			}
			p.write(tw, r, typeInternal, "An unexpected error occurred.", nil)
		}()
		next.ServeHTTP(tw, r)
	})
}

// trackingWriter remembers whether the response has started and its status.
type trackingWriter struct {
	http.ResponseWriter
	wroteHeader bool
	status      int
}

func (t *trackingWriter) WriteHeader(status int) {
	t.wroteHeader = true
	t.status = status
	t.ResponseWriter.WriteHeader(status)
}

func (t *trackingWriter) Write(b []byte) (int, error) {
	if !t.wroteHeader {
		t.WriteHeader(http.StatusOK)
	}
	return t.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (t *trackingWriter) Unwrap() http.ResponseWriter { return t.ResponseWriter }
