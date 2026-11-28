package main

import (
 "context"
 "log"
 "time"
)
func enqueueNotification(s *State,text string){
 s.NextNotificationID++
 s.Notifications=append(s.Notifications,Notification{ID:s.NextNotificationID,Text:text})
}
// DeliverNotifications is a single FIFO worker. Delivery is at least once:
// a crash after Telegram accepts a message but before state commit may duplicate it.
func(a *App) DeliverNotifications(ctx context.Context){
 delay:=time.Second
 for ctx.Err()==nil{
  queue:=a.Store.Snapshot().Notifications
  if len(queue)==0{delay=time.Second}else{
   n:=queue[0]
   if _,e:=a.TG.Send(ctx,a.Config.ChatID,n.Text,nil,false);e!=nil{
    log.Print(e)
    if delay<30*time.Second{delay*=2;if delay>30*time.Second{delay=30*time.Second}}
   }else{
    // Retry persistence without resending during this process lifetime.
    for ctx.Err()==nil{
     e:=a.Store.Update(func(s *State)error{
      if len(s.Notifications)>0&&s.Notifications[0].ID==n.ID{s.Notifications=s.Notifications[1:]}
      return nil
     })
     if e==nil{break}
     log.Print(e)
     select{case <-ctx.Done():return;case <-time.After(time.Second):}
    }
    delay=time.Second
    continue
   }
  }
  select{case <-ctx.Done():return;case <-time.After(delay):}
 }
}
