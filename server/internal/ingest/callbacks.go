package ingest

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

type callback struct { ID int64; URL string; Action []byte; Attempts int }

// DispatchOne retries the oldest due callback. A successful callback is delivered at least once.
func DispatchOne(ctx context.Context, db *sql.DB, client *http.Client) (bool,error) {
	var c callback
	err:=db.QueryRowContext(ctx,`SELECT callback_id,callback_url,action,attempts FROM litterbox_claim_source_callback()`).Scan(&c.ID,&c.URL,&c.Action,&c.Attempts)
	if err==sql.ErrNoRows{return false,nil}; if err!=nil{return false,err}
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,c.URL,bytes.NewReader(c.Action)); if err==nil {req.Header.Set("Content-Type","application/json"); req.Header.Set("Idempotency-Key",fmt.Sprintf("litterbox-source-action-%d",c.ID)); var resp *http.Response; resp,err=client.Do(req); if err==nil {resp.Body.Close(); if resp.StatusCode<200||resp.StatusCode>=300 {err=fmt.Errorf("callback status %d",resp.StatusCode)}}}
	if err==nil {_,err=db.ExecContext(ctx,`SELECT litterbox_complete_source_callback($1)`,c.ID); return true,err}
	_,saveErr:=db.ExecContext(ctx,`SELECT litterbox_fail_source_callback($1,$2)`,c.ID,err.Error())
	if saveErr!=nil{return true,saveErr}; return true,nil
}

// RunCallbacks runs one callback polling loop until cancelled.
func RunCallbacks(ctx context.Context, db *sql.DB, client *http.Client) error {
	ticker:=time.NewTicker(time.Second); defer ticker.Stop()
	for { if _,err:=DispatchOne(ctx,db,client); err!=nil{return err}; select {case <-ctx.Done():return ctx.Err(); case <-ticker.C:} }
}

