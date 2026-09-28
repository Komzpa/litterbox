package cards

import (
 "testing"
 "time"
)

func TestSectionAt1500Tbilisi(t *testing.T) {
 loc,err:=time.LoadLocation("Asia/Tbilisi"); if err!=nil { t.Fatal(err) }
 now:=time.Date(2026,9,28,15,0,0,0,loc)
 at:=func(hour,minute int)*time.Time { v:=time.Date(2026,9,28,hour,minute,0,0,loc); return &v }
 cards:=[]Card{
  {ID:"14",Source:"todo",Title:"early",At:at(14,0),Timed:true,State:"open"},
  {ID:"17",Source:"todo",Title:"later",At:at(17,30),Timed:true,State:"open"},
  {ID:"23",Source:"todo",Title:"sleep",At:at(23,0),Timed:true,State:"open"},
  {ID:"u",Source:"todo",Title:"untimed",Timed:false,State:"open"},
 }
 got:=Section(cards,now)
 if len(got.Now)!=2 || got.Now[0].ID!="14" || got.Now[1].ID!="u" { t.Fatalf("now=%+v",got.Now) }
 if len(got.Later)!=2 || got.Later[0].ID!="17" || got.Later[1].ID!="23" { t.Fatalf("later=%+v",got.Later) }
 if len(got.Missed)!=0 { t.Fatalf("missed=%+v",got.Missed) }
 // A newly started slot supersedes the previous slot; the previous item is missed.
 got=Section(append(cards,Card{ID:"1530",Source:"todo",Title:"started",At:at(14,30),Timed:true,State:"open"}),now)
 if len(got.Now)!=2 || got.Now[0].ID!="1530" || len(got.Missed)!=1 || got.Missed[0].ID!="14" { t.Fatalf("slot transition now=%+v missed=%+v",got.Now,got.Missed) }
}
