package scenarios

import (
	"net/http"
	"testing"
	"time"

	"github.com/massimilianoBastia/geo-ip-allowlist-service/internal/geoipdb"
)

func TestOperationalEndpoints(t *testing.T) {
	db, err := geoipdb.Open(testDBPath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	built := db.Info().BuiltAt.UTC().Format(time.RFC3339)
	s := newScenario(t)

	t.Run("readyz", func(t *testing.T) {
		rec := s.do(http.MethodGet, "/readyz", "", "")
		body := decode(t, rec)
		if rec.Code != 200 || body["status"] != "ready" || body["database_type"] != "GeoIP2-Country" || body["database_built_at"] != built {
			t.Errorf("/readyz = %d %v, want 200 ready GeoIP2-Country %s", rec.Code, body, built)
		}
	})
	t.Run("version", func(t *testing.T) {
		rec := s.do(http.MethodGet, "/version", "", "")
		body := decode(t, rec)
		if rec.Code != 200 || body["version"] != "test" || body["database_built_at"] != built {
			t.Errorf("/version = %d %v, want 200 test %s", rec.Code, body, built)
		}
	})
	t.Run("livez", func(t *testing.T) {
		rec := s.do(http.MethodGet, "/livez", "", "")
		if rec.Code != 200 || decode(t, rec)["status"] != "ok" {
			t.Errorf("/livez = %d %s, want 200 ok", rec.Code, rec.Body)
		}
	})
}
