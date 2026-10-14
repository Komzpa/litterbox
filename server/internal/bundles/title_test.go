package bundles

import "testing"

func TestHumanTitle(t *testing.T) {
 members:=[]TitleMember{{Subject:"September issue",Sender:"News <news@example.test>"},{Subject:"October issue",Sender:"news@example.test"}}
 for _, title:=range []string{"", "September issue", "October issue", "sender:example.test"} {if got:=HumanTitle(title,"seed",members);got!="example.test" {t.Fatalf("%q: %q",title,got)}}
 for _, origin:=range []string{"unknown","explicit","topic"} {
  for _, title:=range []string{"Wireless module supply","September issue","sender:example.test"} {
   if got:=HumanTitle(title,origin,members);got!=title {t.Fatalf("%s topic changed: %q",origin,got)}
  }
 }
 members=append(members,TitleMember{Subject:"Different topic",Sender:"other@second.test"})
 if got:=HumanTitle("September issue","seed",members);got!="example.test, second.test" {t.Fatalf("mixed senders: %q",got)}
}
