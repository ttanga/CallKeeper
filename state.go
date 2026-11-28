package main

import (
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"
 "sync"
)
type PhoneState struct {
 Down *float64 `json:"down"`
 Alerted bool `json:"alerted"`
}
type HistoryEntry struct {
 ID string `json:"id"`
 Started float64 `json:"started"`
 Ended float64 `json:"ended"`
}
type Notification struct {
 ID int64 `json:"id"`
 Text string `json:"text"`
}
type State struct {
 Notifications []Notification `json:"notifications"`
 NextNotificationID int64 `json:"next_notification_id"`
 Version int `json:"version"`
 Devices map[string]string `json:"devices"`
 Settings map[string]string `json:"settings"`
 Phones map[string]PhoneState `json:"phones"`
 History []HistoryEntry `json:"history"`
 Deletions map[int64]float64 `json:"deletions"`
 Offset int64 `json:"offset"`
}
type Store struct {
 mu sync.RWMutex
 path string
 state State
}
func newState() State {return State{Version:1,Devices:map[string]string{},Settings:map[string]string{},Phones:map[string]PhoneState{},History:[]HistoryEntry{},Deletions:map[int64]float64{}}}
func OpenStore(path string)(*Store,error){
 s:=&Store{path:path,state:newState()}
 b,e:=os.ReadFile(path)
 if os.IsNotExist(e){return s,nil}
 if e!=nil{return nil,e}
 if e=json.Unmarshal(b,&s.state);e!=nil{return nil,fmt.Errorf("invalid state JSON: %w",e)}
 if s.state.Version!=1{return nil,fmt.Errorf("unsupported state version")}
 if s.state.Devices==nil||s.state.Settings==nil||s.state.Phones==nil||s.state.Deletions==nil{return nil,fmt.Errorf("missing state maps")}
 return s,nil
}
func cloneState(v State) State {
 b,e:=json.Marshal(v);if e!=nil{panic(e)}
 var n State
 if e=json.Unmarshal(b,&n);e!=nil{panic(e)}
 return n
}
func(s *Store) Snapshot() State {s.mu.RLock();defer s.mu.RUnlock();return cloneState(s.state)}
// Update serializes mutations; callbacks must not perform network I/O or reenter Store.
func(s *Store) Update(fn func(*State)error) error {
 s.mu.Lock();defer s.mu.Unlock()
 n:=cloneState(s.state)
 if e:=fn(&n);e!=nil{return e}
 if len(n.History)>1000{n.History=n.History[len(n.History)-1000:]}
 b,e:=json.MarshalIndent(n,"","  ");if e!=nil{return e}
 if e=atomicWrite(s.path,b);e!=nil{return e}
 s.state=n
 return nil
}
func atomicWrite(path string,b []byte) error {
 dir:=filepath.Dir(path)
 if e:=os.MkdirAll(dir,0700);e!=nil{return e}
 d,e:=os.Open(dir);if e!=nil{return e};defer d.Close()
 f,e:=os.CreateTemp(dir,".state-*");if e!=nil{return e}
 defer os.Remove(f.Name());defer f.Close()
 if _,e=f.Write(b);e!=nil{return e}
 if e=f.Sync();e!=nil{return e}
 if e=f.Close();e!=nil{return e}
 if e=os.Rename(f.Name(),path);e!=nil{return e}
 // Rename is the commit point. Directory sync is best effort.
 _=d.Sync()
 return nil
}
