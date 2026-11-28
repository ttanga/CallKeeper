package main
import("testing";"path/filepath";"sync";"errors";"os")
func TestStore(t *testing.T){
 p:=filepath.Join(t.TempDir(),"state.json")
 s,e:=OpenStore(p);if e!=nil{t.Fatal(e)}
 var wg sync.WaitGroup
 for i:=0;i<20;i++{wg.Add(1);go func(){defer wg.Done();if e:=s.Update(func(v *State)error{v.Offset++;return nil});e!=nil{t.Error(e)}}()}
 wg.Wait()
 r,e:=OpenStore(p);if e!=nil{t.Fatal(e)}
 if r.Snapshot().Offset!=20{t.Fatal("lost updates")}
 v:=r.Snapshot();v.Settings["x"]="y";if r.Snapshot().Settings["x"]!=""{t.Fatal("aliased snapshot")}
 if e=r.Update(func(v *State)error{v.Offset=0;return errors.New("abort")});e==nil||r.Snapshot().Offset!=20{t.Fatal("failed rollback")}
 info,e:=os.Stat(p);if e!=nil{t.Fatal(e)};if info.Mode().Perm()!=0600{t.Fatal("unsafe file mode")}
}
func TestCorruptState(t *testing.T){p:=filepath.Join(t.TempDir(),"state.json");if e:=os.WriteFile(p,[]byte("{"),0600);e!=nil{t.Fatal(e)};if _,e:=OpenStore(p);e==nil{t.Fatal("accepted corrupt state")}}
