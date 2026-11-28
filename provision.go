package main

import (
 "context"
 "crypto/rand"
 "encoding/hex"
 "encoding/json"
 "fmt"
 "io"
 "log"
 "net"
 "strings"
 "time"
 "unicode"
 "unicode/utf8"
)

const provisionSocket="/run/callkeeper-provision.sock"
func provision(ctx context.Context,password string)(string,error){
 ctx,cancel:=context.WithTimeout(ctx,90*time.Second);defer cancel()
 conn,e:=(&net.Dialer{}).DialContext(ctx,"unix",provisionSocket)
 if e!=nil{return "",fmt.Errorf("provisioning service unavailable")}
 defer conn.Close()
 stop:=context.AfterFunc(ctx,func(){conn.Close()});defer stop()
 deadline,_:=ctx.Deadline();if e=conn.SetDeadline(deadline);e!=nil{return "",e}
 b,e:=json.Marshal(map[string]string{"password":password});if e!=nil{return "",e}
 if _,e=conn.Write(b);e!=nil{return "",fmt.Errorf("provisioning request failed")}
 if e=conn.(*net.UnixConn).CloseWrite();e!=nil{return "",fmt.Errorf("provisioning request failed")}
 out,e:=io.ReadAll(io.LimitReader(conn,128))
 id:=strings.TrimSpace(string(out))
 if e!=nil||!sipID.MatchString(id){return "",fmt.Errorf("provisioning not confirmed; inspect PBX locally")}
 return id,nil
}
func(a *App) CreatePhone(ctx context.Context,name string)error{
 if !a.Config.ProvisionEnabled{return fmt.Errorf("provisioning disabled")}
 name=strings.TrimSpace(name)
 if !utf8.ValidString(name)||utf8.RuneCountInString(name)<1||utf8.RuneCountInString(name)>60||strings.IndexFunc(name,unicode.IsControl)>=0{return fmt.Errorf("invalid phone name")}
 var secret [32]byte
 if _,e:=rand.Read(secret[:]);e!=nil{return e}
 password:=hex.EncodeToString(secret[:])
 id,e:=provision(ctx,password);if e!=nil{return e}
 if e=a.Store.Update(func(s *State)error{
  if _,ok:=a.Config.Phones[id];ok{return fmt.Errorf("phone ID collision")}
  if _,ok:=s.Devices[id];ok{return fmt.Errorf("phone ID collision")}
  s.Devices[id]=name;return nil
 });e!=nil{return fmt.Errorf("PBX account created but state not saved; inspect locally")}
 msg,e:=a.TG.Send(ctx,a.Config.ChatID,fmt.Sprintf("%s — %s\nСервер: %s:%d\nTLS / SRTP\nПароль: %s",name,id,a.Config.SIPHost,a.Config.SIPPort,password),nil,true)
 if e!=nil{return fmt.Errorf("account created but credential delivery not confirmed; reset password locally")}
 due:=float64(time.Now().Add(time.Duration(a.Config.PasswordMessageTTL)*time.Second).UnixNano())/1e9
 if e=a.Store.Update(func(s *State)error{s.Deletions[msg.ID]=due;return nil});e!=nil{
  if de:=a.TG.Delete(ctx,a.Config.ChatID,msg.ID);de!=nil{log.Print("credential deletion not confirmed")}
  return fmt.Errorf("credential cleanup scheduling failed")
 }
 a.Menu(ctx)
 return nil
}
func(a *App) Cleanup(ctx context.Context){
 ticker:=time.NewTicker(2*time.Second);defer ticker.Stop()
 for {
  select{case <-ctx.Done():return;case <-ticker.C:}
  now:=float64(time.Now().UnixNano())/1e9
  for id,due:=range a.Store.Snapshot().Deletions{
   if due>now{continue}
   if e:=a.TG.Delete(ctx,a.Config.ChatID,id);e!=nil{log.Print(e);continue}
   if e:=a.Store.Update(func(s *State)error{delete(s.Deletions,id);return nil});e!=nil{log.Print(e)}
  }
 }
}
