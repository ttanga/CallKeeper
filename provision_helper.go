package main

import (
 "context"
 "encoding/json"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "regexp"
 "strings"
 "syscall"
 "time"
)

const managedDir="/etc/asterisk/callkeeper"
const managedFile=managedDir+"/phones.conf"
func provisionHelper()error{
 var req struct{Password string `json:"password"`}
 if e:=json.NewDecoder(io.LimitReader(os.Stdin,1024)).Decode(&req);e!=nil{return fmt.Errorf("invalid request")}
 if !regexp.MustCompile("^[a-f0-9]{64}$").MatchString(req.Password){return fmt.Errorf("invalid password")}
 lock,e:=os.OpenFile(filepath.Join(managedDir,".lock"),os.O_CREATE|os.O_RDWR,0600);if e!=nil{return e};defer lock.Close()
 if e=syscall.Flock(int(lock.Fd()),syscall.LOCK_EX|syscall.LOCK_NB);e!=nil{return fmt.Errorf("provisioning busy")}
 if _,e=os.Stat("/etc/asterisk/callkeeper-provision.enabled");e!=nil{return fmt.Errorf("operator approval missing")}
 main,e:=os.ReadFile("/etc/asterisk/pjsip.conf");if e!=nil{return e}
 if !regexp.MustCompile("(?m)^\\s*#include\\s+\"?callkeeper/phones\\.conf\"?\\s*$").Match(main){return fmt.Errorf("managed include missing")}
 old,e:=os.ReadFile(managedFile);if e!=nil{return e}
 ctx,cancel:=context.WithTimeout(context.Background(),70*time.Second);defer cancel()
 p:=PBX{}
 route,e:=p.CLI(ctx,"dialplan show 2000@from-gateway");if e!=nil||!strings.Contains(route,"gateway_allowed"){return fmt.Errorf("dynamic route missing")}
 transports,e:=p.CLI(ctx,"pjsip show transports");if e!=nil||!strings.Contains(transports,"transport-tls"){return fmt.Errorf("TLS missing")}
 used:=string(old)+string(main)
 for _,cmd:=range []string{"pjsip show endpoints","pjsip show aors","pjsip show auths"}{
  out,e:=p.CLI(ctx,cmd);if e!=nil||!strings.Contains(out,"Objects found:"){return fmt.Errorf("cannot enumerate SIP objects")};used+=out
 }
 id:=""
 for i:=1004;i<2000;i++{k:=fmt.Sprint(i);if !regexp.MustCompile("(^|[^0-9])"+k+"([^0-9]|$)").MatchString(used){id=k;break}}
 if id==""{return fmt.Errorf("no free SIP ID")}
 block:=fmt.Sprintf("\n[%s]\ntype=endpoint\ntransport=transport-tls\ncontext=from-client\ndisallow=all\nallow=alaw,ulaw\ndirect_media=no\nrtp_symmetric=yes\nforce_rport=yes\nrewrite_contact=yes\nrtp_keepalive=15\nrtp_timeout=60\nallow_transfer=no\nallow_subscribe=no\nmedia_encryption=sdes\nmedia_encryption_optimistic=no\nauth=ckauth%s\naors=%s\n[%s]\ntype=aor\nmax_contacts=1\nremove_existing=yes\nqualify_frequency=25\nqualify_timeout=20\ndefault_expiration=120\n[ckauth%s]\ntype=auth\nauth_type=userpass\nusername=%s\npassword=%s\n",id,id,id,id,id,id,req.Password)
 if e=atomicWrite(managedFile,append(old,[]byte(block)...));e!=nil{return e}
 success:=false
 defer func(){if !success{
  rollback,c:=context.WithTimeout(context.Background(),25*time.Second);defer c()
  _,_=p.CLI(rollback,"database del gateway_allowed "+id)
  if atomicWrite(managedFile,old)==nil{_,_=p.CLI(rollback,"pjsip reload")}
 }}()
 if _,e=p.CLI(ctx,"pjsip reload");e!=nil{return e}
 out,e:=p.CLI(ctx,"pjsip show endpoint "+id)
 if e!=nil||!regexp.MustCompile("Endpoint:\\s+"+id+"(?:\\s|/)").MatchString(out)||!strings.Contains(out,"transport-tls"){return fmt.Errorf("endpoint verification failed")}
 if _,e=p.CLI(ctx,"database put gateway_allowed "+id+" 1");e!=nil{return e}
 out,e=p.CLI(ctx,"database get gateway_allowed "+id)
 if e!=nil||!regexp.MustCompile("(?m)Value:\\s*1\\s*$").MatchString(out){return fmt.Errorf("route authorization failed")}
 success=true
 fmt.Println(id)
 return nil
}
