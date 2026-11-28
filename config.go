package main

import (
 "encoding/json"
 "fmt"
 "os"
 "regexp"
 "strings"
)

type Phone struct {
 Name string `json:"name"`
 Target bool `json:"target"`
}
type Config struct {
 BotToken string `json:"bot_token"`
 ChatID int64 `json:"chat_id"`
 UserIDs []int64 `json:"user_ids"`
 PollSeconds int `json:"poll_seconds"`
 DownDelay int `json:"down_delay"`
 MonitorEnabled bool `json:"monitor_enabled"`
 DefaultTarget string `json:"default_target"`
 Phones map[string]Phone `json:"phones"`
 ProvisionEnabled bool `json:"provision_enabled"`
 SIPHost string `json:"sip_host"`
 SIPPort int `json:"sip_port"`
 PasswordMessageTTL int `json:"password_message_ttl"`
}
var sipID = regexp.MustCompile("^[0-9]{1,10}$")
func LoadConfig(path string) (Config,error) {
 c:=Config{PollSeconds:3,DownDelay:60,SIPPort:35061,PasswordMessageTTL:60}
 b,e:=os.ReadFile(path); if e!=nil{return c,e}
 if e=json.Unmarshal(b,&c);e!=nil{return c,fmt.Errorf("invalid config JSON")}
 return c,c.Validate()
}
func(c Config) Validate() error {
 if strings.TrimSpace(c.BotToken)=="" || c.BotToken=="REPLACE" || c.ChatID==0 || len(c.UserIDs)==0 {return fmt.Errorf("configure Telegram credentials and authorized IDs")}
 for _,id:=range c.UserIDs {if id<=0{return fmt.Errorf("invalid authorized user ID")}}
 if c.PollSeconds<1 || c.DownDelay<1 || c.PasswordMessageTTL<1 || c.SIPPort<1 || c.SIPPort>65535{return fmt.Errorf("invalid timing or SIP port")}
 if !c.Phones[c.DefaultTarget].Target{return fmt.Errorf("invalid default target")}
 for id,p:=range c.Phones {if !sipID.MatchString(id)||strings.TrimSpace(p.Name)==""{return fmt.Errorf("invalid phone configuration")}}
 if c.ProvisionEnabled && strings.TrimSpace(c.SIPHost)==""{return fmt.Errorf("missing SIP host")}
 return nil
}
