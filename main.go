package main

import (
 "context"
 "flag"
 "fmt"
 "log"
 "os"
 "os/signal"
 "path/filepath"
 "sync"
 "syscall"
)
func main(){
 root:=flag.String("root","/apps/callkeeper","application directory")
 check:=flag.Bool("check",false,"validate configuration and JSON state")
 flag.Parse()
 c,e:=LoadConfig(filepath.Join(*root,"config.json"));if e!=nil{log.Fatal(e)}
 path:=filepath.Join(*root,"state","state.json")
 s,e:=OpenStore(path);if e!=nil{log.Fatal(e)}
 if *check{fmt.Println("Configuration and JSON state validation passed");return}
 if c.ProvisionEnabled{log.Fatal("Provisioning must remain disabled until the helper and dialplan are installed")}
 lock,e:=os.OpenFile(filepath.Join(*root,"state",".go.lock"),os.O_CREATE|os.O_RDWR,0600)
 if e!=nil{log.Fatal(e)}
 defer lock.Close()
 if e=syscall.Flock(int(lock.Fd()),syscall.LOCK_EX|syscall.LOCK_NB);e!=nil{log.Fatal("Another Go CallKeeper instance is running")}
 // Reopen after acquiring the lock to avoid using a stale snapshot.
 s,e=OpenStore(path);if e!=nil{log.Fatal(e)}
 if e=s.Update(func(*State)error{return nil});e!=nil{log.Fatal(e)}
 ctx,cancel:=signal.NotifyContext(context.Background(),os.Interrupt,syscall.SIGTERM);defer cancel()
 t:=NewTelegram(c.BotToken)
 a:=NewApp(c,s,t)
 var wg sync.WaitGroup
 for _,worker:=range []func(context.Context){a.Monitor,a.Cleanup,a.DeliverNotifications}{
  wg.Add(1);go func(fn func(context.Context)){defer wg.Done();fn(ctx)}(worker)
 }
 e=t.Run(ctx,c,s,a.Handle)
 cancel();wg.Wait()
 if e!=nil&&e!=context.Canceled{log.Fatal(e)}
}
