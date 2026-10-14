package bundles

import "testing"

func TestHumanTitle(t *testing.T) {
 members:=[]TitleMember{{Subject:"September issue",Sender:"News <news@example.test>"},{Subject:"October issue",Sender:"news@example.test"}}
 for _, title:=range []string{"", "September issue", "October issue", "sender:example.test"} {if got:=HumanTitle(title,members);got!="example.test" {t.Fatalf("%q: %q",title,got)}}
 if got:=HumanTitle("Wireless module supply",members);got!="Wireless module supply" {t.Fatalf("human topic changed: %q",got)}
 members=append(members,TitleMember{Subject:"Different topic",Sender:"other@second.test"})
 if got:=HumanTitle("September issue",members);got!="example.test, second.test" {t.Fatalf("mixed senders: %q",got)}
}
