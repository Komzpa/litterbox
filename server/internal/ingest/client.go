package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Post sends one card through the shared ingest API using a source-scoped token.
func Post(ctx context.Context, endpoint, token string, req Request) error {
	if endpoint=="" || token=="" { return fmt.Errorf("ingest endpoint and source token are required") }
	if strings.HasSuffix(endpoint,"/") { endpoint=strings.TrimSuffix(endpoint,"/") }
	body,err:=json.Marshal(req); if err!=nil{return err}
	httpReq,err:=http.NewRequestWithContext(ctx,http.MethodPost,endpoint+"/v1/ingest",bytes.NewReader(body)); if err!=nil{return err}
	httpReq.Header.Set("Authorization","Bearer "+token); httpReq.Header.Set("Content-Type","application/json")
	client:=&http.Client{Timeout:10*time.Second}; response,err:=client.Do(httpReq); if err!=nil{return err}; defer response.Body.Close()
	if response.StatusCode<200 || response.StatusCode>=300 {return fmt.Errorf("ingest API returned %s",response.Status)}
	return nil
}
