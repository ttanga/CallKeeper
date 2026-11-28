package main

import (
 "context"
 "fmt"
 "log"
 "sort"
 "strconv"
 "strings"
 "time"
)
func(a *App) monitoring(s State)bool{
 if v,ok:=s.Settings["monitor"];ok{return v=="1"}
 return a.Config.MonitorEnabled
}
func(a *App) delay(s State)int{
 if n,e:=strconv.Atoi(s.Settings["delay"]);e==nil&&n>0{return n}
 return a.Config.DownDelay
}
func notifyEnabled(s State,id string)bool{return s.Settings["notify:"+id]!="0"}
func(a *App) label(id string)string{if p,ok:=a.Phones()[id];ok{return id+" — "+p.Name};return id}
func(a *App) observe(ctx context.Context,id string,up bool,now float64){
 s:=a.Store.Snapshot()
 if !a.monitoring(s){return}
 old:=s.Phones[id]
 if up&&old.Down==nil{return}
 if !up&&old.Down==nil{
  if e:=a.Store.Update(func(v *State)error{v.Phones[id]=PhoneState{Down:&now};return nil});e!=nil{log.Print(e)}
  return
 }
 if !up&&(old.Alerted||!notifyEnabled(s,id)||now-*old.Down<float64(a.delay(s))){return}
 label:=a.label(id)
 if e:=a.Store.Update(func(v *State)error{
  if !a.monitoring(*v){return nil}
  p:=v.Phones[id]
  if p.Down==nil{return nil}
  if up{
   if p.Alerted{enqueueNotification(v,fmt.Sprintf("🟢 Восстановлен: %s; %d сек.",label,int(now-*p.Down)))}
   v.History=append(v.History,HistoryEntry{ID:id,Started:*p.Down,Ended:now})
   delete(v.Phones,id)
  }else if !p.Alerted&&notifyEnabled(*v,id)&&now-*p.Down>=float64(a.delay(*v)){
   enqueueNotification(v,"🔴 Недоступен: "+label)
   p.Alerted=true;v.Phones[id]=p
  }
  return nil
 });e!=nil{log.Print(e)}
}
func(a *App) Monitor(ctx context.Context){
 ticker:=time.NewTicker(time.Duration(a.Config.PollSeconds)*time.Second);defer ticker.Stop()
 for {
  if ctx.Err()!=nil{return}
  if a.monitoring(a.Store.Snapshot()){
   statuses,e:=a.PBX.Contacts(ctx,a.Phones())
   now:=float64(time.Now().UnixNano())/1e9
   a.observe(ctx,"Asterisk",e==nil,now)
   if e==nil{for id,up:=range statuses{a.observe(ctx,id,up,now)}}
  }
  select{case <-ctx.Done():return;case <-ticker.C:}
 }
}
func(a *App) Settings(ctx context.Context){
 s:=a.Store.Snapshot()
 ids:=[]string{"Asterisk"};for id:=range a.Phones(){ids=append(ids,id)};sort.Strings(ids[1:])
 buttons:=[][]TGButton{}
 for _,id:=range ids{mark:="🔕 ";if notifyEnabled(s,id){mark="🔔 "};buttons=append(buttons,[]TGButton{{Text:mark+a.label(id),Data:"notify:"+id}})}
 delays:=[]TGButton{}
 for _,n:=range []int{30,60,120,300}{label:=fmt.Sprintf("%d сек.",n);if n==a.delay(s){label="✓ "+label};delays=append(delays,TGButton{Text:label,Data:fmt.Sprintf("delay:%d",n)})}
 buttons=append(buttons,delays,[]TGButton{{Text:"Мониторинг вкл.",Data:"monitor:1"},{Text:"Мониторинг выкл.",Data:"monitor:0"}},[]TGButton{{Text:"Назад",Data:"menu"}})
 status:="выключен";if a.monitoring(s){status="включён"}
 a.send(ctx,fmt.Sprintf("Задержка: %d сек.\nМониторинг: %s",a.delay(s),status),buttons)
}
func(a *App) History(ctx context.Context){
 h:=a.Store.Snapshot().History
 lines:=[]string{}
 for i:=len(h)-1;i>=0&&len(lines)<15;i--{v:=h[i];lines=append(lines,fmt.Sprintf("%s | %s | %d сек.",a.label(v.ID),time.Unix(int64(v.Started),0).Format("2006-01-02 15:04"),int(v.Ended-v.Started)))}
 if len(lines)==0{lines=append(lines,"История пуста")}
 a.send(ctx,strings.Join(lines,"\n"),nil)
}
func(a *App) ChangeSetting(ctx context.Context,action string)error{
 parts:=strings.SplitN(action,":",2);if len(parts)!=2{return fmt.Errorf("invalid setting")}
 key,value:=parts[0],parts[1]
 e:=a.Store.Update(func(s *State)error{
  switch key{
  case "notify":
   if value!="Asterisk"{if _,ok:=a.Config.Phones[value];!ok{if _,ok=s.Devices[value];!ok{return fmt.Errorf("unknown phone")}}}
   v:="1";if notifyEnabled(*s,value){v="0"};s.Settings["notify:"+value]=v
  case "delay":
   switch value{case "30","60","120","300":s.Settings["delay"]=value;default:return fmt.Errorf("invalid delay")}
  case "monitor":
   if value!="0"&&value!="1"{return fmt.Errorf("invalid monitor setting")}
   s.Settings["monitor"]=value
  default:return fmt.Errorf("unknown setting")
  }
  return nil
 })
 if e==nil{a.Settings(ctx)}
 return e
}
