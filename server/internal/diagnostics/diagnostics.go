// Package diagnostics receives only app-sanitized crash evidence, never raw logs.
package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/Komzpa/litterbox/server/internal/auth"
	"github.com/jackc/pgx/v5"
)

const Path = "/v1/diagnostics/crashes"
const MaxBody = 65536

var (
	reportID     = regexp.MustCompile(`^[a-z0-9-]{1,96}$`)
	buildAtom    = regexp.MustCompile(`^[A-Za-z0-9._+-]{1,64}$`)
	classLine    = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*(\.[A-Za-z_$][A-Za-z0-9_$]*)+$`)
	signedNumber = regexp.MustCompile(`^-?[0-9]{1,10}$`)
	frameLine    = regexp.MustCompile(`^  at [A-Za-z0-9_$.<>-]{1,512}:-?[0-9]{1,9}$`)
	number       = regexp.MustCompile(`^[0-9]{1,19}$`)
	versionAtom  = regexp.MustCompile(`^[A-Za-z0-9._+-]{1,96}$`)
)

// Report preserves the historical client envelope. Attribution is never supplied
// by the caller: it comes exclusively from the authenticated device context.
type Report struct {
	ID            string `json:"report_id"`
	Body          string `json:"body"`
	UploaderBuild string `json:"uploader_build"`
}

type Store interface {
	Save(context.Context, auth.Device, Report, string, string) error
}

type Handler struct{ Store Store }

func Register(mux *http.ServeMux, store Store) {
	mux.Handle("POST "+Path, Handler{Store: store})
}

// Validate rejects unknown/duplicate metadata, messages and raw traces. The
// accepted grammar is deliberately narrower than arbitrary exception text.
func Validate(r Report) (kind, version string, err error) {
	invalid := errors.New("invalid sanitized report")
	if !reportID.MatchString(r.ID) || !buildAtom.MatchString(r.UploaderBuild) || len(r.Body) == 0 || len(r.Body) > MaxBody {
		return "", "", invalid
	}
	meta := make(map[string]string)
	hasFrames := false
	for _, line := range strings.Split(strings.TrimSuffix(r.Body, "\n"), "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			if _, exists := meta[key]; exists {
				return "", "", invalid
			}
			valid := false
			switch key {
			case "schema":
				valid = value == "1"
			case "version":
				valid = versionAtom.MatchString(value)
			case "build":
				valid = number.MatchString(value) || value == "unknown"
			case "android", "timestamp":
				valid = number.MatchString(value)
			case "reason":
				valid = number.MatchString(value) || value == "uncaught" || value == "unavailable"
			case "status":
				valid = signedNumber.MatchString(value) || value == "unknown"
			case "kind":
				valid = value == "java" || value == "exit"
			case "stack":
				valid = value == "unavailable" || value == "frames"
			case "version_scope":
				valid = value == "current-process" || value == "unknown-historical-process"
			case "package":
				valid = value == "org.qtproject.example.litterbox_qt"
			case "history":
				valid = value == "available" || value == "empty" || value == "unavailable" || value == "unsupported"
			}
			if !valid {
				return "", "", invalid
			}
			meta[key] = value
		} else if !frameLine.MatchString(line) && !classLine.MatchString(line) {
			return "", "", invalid
		} else {
			hasFrames = true
		}
	}
	for _, key := range []string{"schema", "version", "build", "package", "android", "timestamp", "kind", "reason", "status", "stack", "version_scope"} {
		if meta[key] == "" {
			return "", "", invalid
		}
	}
	if meta["kind"] == "exit" {
		if meta["stack"] != "unavailable" || meta["version_scope"] != "unknown-historical-process" || meta["version"] != "unknown" || meta["build"] != "unknown" {
			return "", "", invalid
		}
		if hasFrames || (meta["reason"] != "unavailable" && !number.MatchString(meta["reason"])) {
			return "", "", invalid
		}
		if meta["reason"] == "unavailable" {
			if meta["status"] != "unknown" || meta["history"] == "" {
				return "", "", invalid
			}
		} else if meta["history"] != "" && meta["history"] != "available" {
			return "", "", invalid
		}
	} else if meta["reason"] != "uncaught" || meta["status"] != "unknown" || meta["stack"] != "frames" || meta["version_scope"] != "current-process" || meta["history"] != "" {
		return "", "", invalid
	}
	return meta["kind"], meta["version"], nil
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d, ok := auth.DeviceFromContext(r.Context())
	if !ok || d.TenantID == "" || d.DeviceID == "" || d.DeviceID == "development" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var report Report
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBody*2+4096))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&report); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	kind, version, err := Validate(report)
	if err != nil {
		http.Error(w, "invalid sanitized report", http.StatusUnprocessableEntity)
		return
	}
	if err := h.Store.Save(r.Context(), d, report, kind, version); err != nil {
		http.Error(w, "store failed", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"ack": report.ID})
}

type DB interface {
	Begin(context.Context) (pgx.Tx, error)
}
type PostgresStore struct{ DB DB }

func (s PostgresStore) Save(ctx context.Context, d auth.Device, r Report, kind, version string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT set_config('litterbox.tenant_id',$1,true), set_config('litterbox.device_id',$2,true)`, d.TenantID, d.DeviceID); err != nil {
		return err
	}
	// Hold the device row through commit, serializing with RevokeDevice's UPDATE.
	// A completed revoke wins the check; a save holding the lock commits first.
	var active bool
	if err = tx.QueryRow(ctx, `SELECT true FROM devices WHERE tenant_id=$1::uuid AND id=$2::uuid AND revoked_at IS NULL FOR UPDATE`, d.TenantID, d.DeviceID).Scan(&active); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO crash_reports (tenant_id,device_id,report_id,kind,app_version,uploader_build,body)
		VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6,$7)
		ON CONFLICT (tenant_id,device_id,report_id) DO NOTHING`, d.TenantID, d.DeviceID, r.ID, kind, version, r.UploaderBuild, r.Body); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
