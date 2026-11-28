package main

import (
 "bytes"
 "context"
 "encoding/json"
 "fmt"
 "io"
 "log"
 "net"
 "net/http"
 "time"
)

type Telegram struct {
 base string
 client *http.Client
}
type TGUser struct { ID int64 `json:"id"` }
type TGChat struct { ID int64 `json:"id"`; Type string `json:"type"` }
type TGMessage struct { ID int64 `json:"message_id"`; From TGUser `json:"from"`; Chat TGChat `json:"chat"`; Text string `json:"text"`; Date int64 `json:"date"` }
type TGCallback struct { ID string `json:"id"`; From TGUser `json:"from"`; Message *TGMessage `json:"message"`; Data string `json:"data"` }
type TGUpdate struct { ID int64 `json:"update_id"`; Message *TGMessage `json:"message"`; Callback *TGCallback `json:"callback_query"` }
type TGButton struct { Text string `json:"text"`; Data string `json:"callback_data"` }

func NewTelegram(token string)*Telegram {
 tr:=http.DefaultTransport.(*http.Transport).Clone()
 tr.DialContext=(&net.Dialer{Timeout:5*time.Second,KeepAlive:30*time.Second}).DialContext
 tr.TLSHandshakeTimeout=5*time.Second
 tr.MaxIdleConnsPerHost=8
 return &Telegram{base:"https://api.telegram.org/bot"+token+"/",client:&http.Client{Transport:tr,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}}
}
func(t *Telegram) call(ctx context.Context,method string,args any,out any,timeout time.Duration) error {
 start:=time.Now()
 defer func(){log.Printf("telegram method=%s duration=%s",method,time.Since(start).Round(time.Millisecond))}()
 ctx,cancel:=context.WithTimeout(ctx,timeout);defer cancel()
 b,e:=json.Marshal(args);if e!=nil{return fmt.Errorf("telegram encode failed")}
 req,e:=http.NewRequestWithContext(ctx,http.MethodPost,t.base+method,bytes.NewReader(b));if e!=nil{return fmt.Errorf("telegram request invalid")}
 req.Header.Set("Content-Type","application/json")
 resp,e:=t.client.Do(req);if e!=nil{return fmt.Errorf("telegram %s transport failure: %T",method,e)}
 defer resp.Body.Close()
 if resp.StatusCode!=200{return fmt.Errorf("telegram %s HTTP %d",method,resp.StatusCode)}
 var envelope struct { OK bool `json:"ok"`; Result json.RawMessage `json:"result"`; ErrorCode int `json:"error_code"` }
 if e=json.NewDecoder(io.LimitReader(resp.Body,8<<20)).Decode(&envelope);e!=nil{return fmt.Errorf("telegram %s invalid response",method)}
 if !envelope.OK{return fmt.Errorf("telegram %s rejected: %d",method,envelope.ErrorCode)}
 if out!=nil {if e=json.Unmarshal(envelope.Result,out);e!=nil{return fmt.Errorf("telegram %s invalid result",method)}}
 return nil
}
func(t *Telegram) Poll(ctx context.Context,offset int64)([]TGUpdate,error){
 var updates []TGUpdate
 e:=t.call(ctx,"getUpdates",map[string]any{"offset":offset,"timeout":25,"limit":50,"allowed_updates":[]string{"message","callback_query"}},&updates,35*time.Second)
 return updates,e
}
func(t *Telegram) Ack(ctx context.Context,id,text string)error{
 return t.call(ctx,"answerCallbackQuery",map[string]any{"callback_query_id":id,"text":text},nil,15*time.Second)
}
func(t *Telegram) Send(ctx context.Context,chat int64,text string,buttons [][]TGButton,protect bool)(TGMessage,error){
 args:=map[string]any{"chat_id":chat,"text":text,"protect_content":protect}
 if len(buttons)>0{args["reply_markup"]=map[string]any{"inline_keyboard":buttons}}
 var msg TGMessage
 e:=t.call(ctx,"sendMessage",args,&msg,15*time.Second)
 return msg,e
}
func(t *Telegram) Delete(ctx context.Context,chat,id int64)error{
 return t.call(ctx,"deleteMessage",map[string]any{"chat_id":chat,"message_id":id},nil,15*time.Second)
}
func(c Config) authorized(user int64,chat TGChat)bool{
 if chat.Type!="private"||chat.ID!=c.ChatID{return false}
 for _,id:=range c.UserIDs{if id==user{return true}}
 return false
}
func(c Config) accepts(u TGUpdate)bool{
 if u.Callback!=nil{return u.Callback.Message!=nil&&c.authorized(u.Callback.From.ID,u.Callback.Message.Chat)}
 return u.Message!=nil&&c.authorized(u.Message.From.ID,u.Message.Chat)
}
// Run receives updates independently of a serial command worker. The bounded
// queue provides backpressure. Offset is persisted before enqueue: at-most-once
// dispatch, with possible loss on crash; never replay a dispatched call.
func(t *Telegram) Run(ctx context.Context,c Config,s *Store,handle func(context.Context,TGUpdate))error{
 jobs:=make(chan TGUpdate,64)
 acks:=make(chan string,64)
 workerDone:=make(chan struct{})
 go func(){defer close(workerDone);for {select{case <-ctx.Done():return;case u:=<-jobs:handle(ctx,u)}}}()
 ackDone:=make(chan struct{})
 go func(){defer close(ackDone);for {select{case <-ctx.Done():return;case id:=<-acks:if e:=t.Ack(ctx,id,"");e!=nil{log.Print(e)}}}}()
 defer func(){<-workerDone;<-ackDone}()
 offset:=s.Snapshot().Offset
 for ctx.Err()==nil {
  updates,e:=t.Poll(ctx,offset)
  if e!=nil{log.Print(e);select{case <-ctx.Done():return ctx.Err();case <-time.After(2*time.Second):};continue}
  for _,u:=range updates{
   if u.ID<offset{continue}
   next:=u.ID+1
   if e=s.Update(func(v *State)error{v.Offset=next;return nil});e!=nil{log.Printf("offset persistence failed: %v",e);break}
   offset=next
   if !c.accepts(u){continue}
   if u.Callback!=nil{select{case acks<-u.Callback.ID:default:log.Print("callback acknowledgement queue full")}}
   select{case jobs<-u:case <-ctx.Done():return ctx.Err()}
  }
 }
 return ctx.Err()
}
