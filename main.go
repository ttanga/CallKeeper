package main

import (
 "flag"
 "fmt"
 "os"
 "path/filepath"
)
func main(){
 root:=flag.String("root","/apps/callkeeper","application directory")
 check:=flag.Bool("check",false,"validate configuration and existing JSON state")
 flag.Parse()
 if !*check {fmt.Fprintln(os.Stderr,"Go migration stage 1: runtime not implemented; use --check");os.Exit(2)}
 if _,e:=LoadConfig(filepath.Join(*root,"config.json"));e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}
 if _,e:=OpenStore(filepath.Join(*root,"state","state.json"));e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}
 fmt.Println("Configuration and JSON state validation passed")
}
