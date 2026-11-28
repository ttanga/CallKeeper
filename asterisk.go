package main

import (
 "context"
 "crypto/rand"
 "encoding/hex"
 "fmt"
 "log"
 "os/exec"
 "regexp"
 "sort"
 "strings"
 "time"
)
type PBX struct{}
func(PBX) CLI(ctx context.Context,command string)(string,error){
 start:=time.Now()
 defer func(){log.Printf("asterisk duration=%s",time.Since(start).Round(time.Millisecond))}()
 ctx,cancel:=context.WithTimeout(ctx,12*time.Second);defer cancel()
 b,e:=exec.CommandContext(ctx,"/usr/sbin/asterisk","-rx",command).CombinedOutput()
 if e!=nil||strings.Contains(string(b),"Unable to connect"){return "",fmt.Errorf("Asterisk CLI unavailable")}
 return string(b),nil
}
var emptyContacts=regexp.MustCompile("Objects found:\\s*0|No objects found")
func parseContacts(out string,phones map[string]Phone)(map[string]bool,error){
 if !strings.Contains(out,"Contact:")&&!emptyContacts.MatchString(out){return nil,fmt.Errorf("unexpected contact output")}
 status:=map[string]bool{}
 for id:=range phones{status[id]=false}
 for _,line:=range strings.Split(out,"\n"){
  f:=strings.Fields(line)
  if len(f)>=5&&f[0]=="Contact:"{
   id:=strings.SplitN(f[1],"/",2)[0]
   if _,ok:=status[id];ok&&f[3]=="Avail"{status[id]=true}
  }
 }
 return status,nil
}
func(p PBX) Contacts(ctx context.Context,phones map[string]Phone)(map[string]bool,error){
 out,e:=p.CLI(ctx,"pjsip show contacts");if e!=nil{return nil,e};return parseContacts(out,phones)
}
var targetRE=regexp.MustCompile("(?m)Value:\\s*([0-9]+)\\s*$")
func(p PBX) Target(ctx context.Context,fallback string)(string,error){
 out,e:=p.CLI(ctx,"database get gateway target");if e!=nil{return "",e}
 if m:=targetRE.FindStringSubmatch(out);len(m)>0{return m[1],nil}
 if strings.Contains(strings.ToLower(out),"not exist"){return fallback,nil}
 return "",fmt.Errorf("cannot read route")
}
func(p PBX) SetTarget(ctx context.Context,id string,phones map[string]Phone)error{
 if !sipID.MatchString(id)||!phones[id].Target{return fmt.Errorf("target not allowed")}
 out,e:=p.CLI(ctx,"pjsip show endpoint "+id);if e!=nil{return e}
 if !regexp.MustCompile("Endpoint:\\s+"+id+"(?:\\s|/)").MatchString(out){return fmt.Errorf("endpoint not found")}
 if !p.RouteAllowed(ctx,id){return fmt.Errorf("target not supported by current dialplan")}
 if _,e=p.CLI(ctx,"database put gateway target "+id);e!=nil{return e}
 actual,e:=p.Target(ctx,"");if e!=nil||actual!=id{return fmt.Errorf("route not confirmed")}
 return nil
}
func(p PBX) Test(ctx context.Context,phones map[string]Phone,fallback string)error{
 id,e:=p.Target(ctx,fallback);if e!=nil{return e}
 if !phones[id].Target||!p.RouteAllowed(ctx,id){return fmt.Errorf("invalid target")}
 out,e:=p.CLI(ctx,"dialplan show 2000@from-gateway");if e!=nil{return e}
 if !regexp.MustCompile("2000\\s*=>|['\"]2000['\"]").MatchString(out){return fmt.Errorf("route 2000 not found")}
 out,e=p.CLI(ctx,"channel originate Local/2000@from-gateway application Echo")
 if e!=nil{return e}
 if strings.Contains(strings.ToLower(out),"failed")||strings.Contains(out,"No such command"){return fmt.Errorf("originate rejected")}
 return nil
}
type Confirmation struct{ User int64; Expires time.Time }
type App struct{
 Config Config
 Store *Store
 TG *Telegram
 PBX PBX
 pendingUser int64
 pendingUntil time.Time
 confirmations map[string]Confirmation
}
func NewApp(c Config,s *Store,t *Telegram)*App{return &App{Config:c,Store:s,TG:t,confirmations:map[string]Confirmation{}}}
func(a *App) Phones()map[string]Phone{
 p:=map[string]Phone{}
 for id,v:=range a.Config.Phones{p[id]=v}
 for id,name:=range a.Store.Snapshot().Devices{if sipID.MatchString(id){if _,exists:=p[id];!exists{p[id]=Phone{Name:name,Target:true}}}}
 return p
}
func(a *App) send(ctx context.Context,text string,buttons [][]TGButton){
 if _,e:=a.TG.Send(ctx,a.Config.ChatID,text,buttons,false);e!=nil{log.Print(e)}
}
func(a *App) Menu(ctx context.Context){
 phones:=a.Phones()
 target,e:=a.PBX.Target(ctx,a.Config.DefaultTarget)
 statuses,se:=a.PBX.Contacts(ctx,phones)
 lines:=[]string{"CallKeeper"}
 if e!=nil||se!=nil{lines=append(lines,"⚠️ Asterisk CLI недоступен или ответ не распознан")}else{lines=append(lines,"🟢 Asterisk","Получатель: "+target+" — "+phones[target].Name)}
 ids:=[]string{};for id:=range phones{ids=append(ids,id)};sort.Strings(ids)
 buttons:=[][]TGButton{}
 for _,id:=range ids{
  p:=phones[id];mark:="⚪";if se==nil{mark="🔴";if statuses[id]{mark="🟢"}}
  lines=append(lines,mark+" "+id+" — "+p.Name)
  if p.Target&&a.PBX.RouteAllowed(ctx,id){label:=id+" — "+p.Name;if id==target{label="✓ "+label};buttons=append(buttons,[]TGButton{{Text:label,Data:"target:"+id}})}
 }
 buttons=append(buttons,[]TGButton{{Text:"Тестовый звонок",Data:"test_confirm"},{Text:"Обновить статус",Data:"menu"}})
 if a.Config.ProvisionEnabled{buttons=append(buttons,[]TGButton{{Text:"Добавить телефон",Data:"add"}})}
 buttons=append(buttons,[]TGButton{{Text:"Уведомления",Data:"settings"},{Text:"История",Data:"history"}})
 a.send(ctx,strings.Join(lines,"\n"),buttons)
}
func(a *App) Handle(ctx context.Context,u TGUpdate){
 if !a.Config.accepts(u){return}
 if u.Callback==nil{
  if u.Message==nil{return}
  switch u.Message.Text{
  case "/start","/menu","/status","/cancel":a.pendingUser=0;a.Menu(ctx)
  default:
   if a.pendingUser==u.Message.From.ID&&time.Now().Before(a.pendingUntil)&&!strings.HasPrefix(u.Message.Text,"/"){
    a.pendingUser=0
    if e:=a.CreatePhone(ctx,u.Message.Text);e!=nil{log.Print(e);a.send(ctx,"⚠️ "+e.Error(),nil)}
   }
  }
  return
 }
 cb:=u.Callback
 now:=time.Now()
 for key,v:=range a.confirmations{if !now.Before(v.Expires){delete(a.confirmations,key)}}
 var e error
 switch {
 case cb.Data=="add":
  if !a.Config.ProvisionEnabled{a.send(ctx,"Создание аккаунтов отключено.",nil);return}
  a.pendingUser=cb.From.ID;a.pendingUntil=now.Add(2*time.Minute)
  a.send(ctx,"Введите название нового телефона или /cancel",nil)
 case cb.Data=="menu":a.pendingUser=0;a.Menu(ctx)
 case cb.Data=="settings":a.Settings(ctx)
 case cb.Data=="history":a.History(ctx)
 case strings.HasPrefix(cb.Data,"notify:")||strings.HasPrefix(cb.Data,"delay:")||strings.HasPrefix(cb.Data,"monitor:"):e=a.ChangeSetting(ctx,cb.Data)
 case strings.HasPrefix(cb.Data,"target:"):
  e=a.PBX.SetTarget(ctx,strings.TrimPrefix(cb.Data,"target:"),a.Phones());if e==nil{a.Menu(ctx)}
 case cb.Data=="test_confirm":
  var b [16]byte
  if _,e=rand.Read(b[:]);e==nil{
   key:=hex.EncodeToString(b[:])
   for k,v:=range a.confirmations{if v.User==cb.From.ID{delete(a.confirmations,k)}}
   a.confirmations[key]=Confirmation{User:cb.From.ID,Expires:now.Add(60*time.Second)}
   a.send(ctx,"Позвонить на текущий маршрут? Подтверждение действует 60 секунд.",[][]TGButton{{{Text:"Подтвердить",Data:"test:"+key},{Text:"Отмена",Data:"cancel:"+key}}})
  }
 case strings.HasPrefix(cb.Data,"cancel:"):
  key:=strings.TrimPrefix(cb.Data,"cancel:");if v,ok:=a.confirmations[key];ok&&v.User==cb.From.ID{delete(a.confirmations,key)};a.Menu(ctx)
 case strings.HasPrefix(cb.Data,"test:"):
  key:=strings.TrimPrefix(cb.Data,"test:");v,ok:=a.confirmations[key]
  if !ok||v.User!=cb.From.ID||!now.Before(v.Expires){a.send(ctx,"Подтверждение истекло или уже использовано.",nil);return}
  delete(a.confirmations,key)
  e=a.PBX.Test(ctx,a.Phones(),a.Config.DefaultTarget)
  if e==nil{a.send(ctx,"Команда тестового звонка отправлена. Проверь телефон.",nil)}
 default:a.send(ctx,"Команда устарела или пока не поддерживается.",nil)
 }
 if e!=nil{log.Printf("command failed: %v",e);a.send(ctx,"⚠️ Команда не выполнена или результат не подтверждён.",nil)}
}

func(p PBX) RouteAllowed(ctx context.Context,id string)bool{
 if id=="1002"||id=="1003"{return true}
 if !sipID.MatchString(id)||id=="1001"{return false}
 route,e:=p.CLI(ctx,"dialplan show 2000@from-gateway")
 if e!=nil||!strings.Contains(route,"gateway_allowed"){return false}
 out,e:=p.CLI(ctx,"database get gateway_allowed "+id)
 return e==nil&&regexp.MustCompile("(?m)Value:\\s*1\\s*$").MatchString(out)
}
