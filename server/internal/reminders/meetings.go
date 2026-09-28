package reminders

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// UpsertMeeting is the meeting-kind ingest hook. endAt is the calendar event's
// scheduled end; only the Litterbox card is changed when it passes or is done.
func UpsertMeeting(ctx context.Context, db *sql.DB, tenant, externalID, title string, endAt time.Time) error {
	if tenant == "" || strings.TrimSpace(externalID) == "" || strings.TrimSpace(title) == "" || endAt.IsZero() {
		return fmt.Errorf("meeting tenant, external id, title and end time are required")
	}
	tx, err := db.BeginTx(ctx,nil); if err != nil { return err }; defer tx.Rollback()
	if _,err=tx.ExecContext(ctx,`SELECT set_config('litterbox.tenant_id',$1,true)`,tenant); err!=nil{return err}
	_,err=tx.ExecContext(ctx,`INSERT INTO cards(tenant_id,id,source,external_id,title,summary,at,timed,state,sort_at) VALUES($1,gen_random_uuid(),'meeting',$2,$3,'',$4,true,'open',$4) ON CONFLICT (tenant_id,source,external_id) DO UPDATE SET title=EXCLUDED.title,at=EXCLUDED.at,timed=true,sort_at=EXCLUDED.sort_at,state=CASE WHEN cards.state='done' THEN 'done' ELSE 'open' END`,tenant,strings.TrimSpace(externalID),strings.TrimSpace(title),endAt.UTC())
	if err!=nil{return err}; return tx.Commit()
}
