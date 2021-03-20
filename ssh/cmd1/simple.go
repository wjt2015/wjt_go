package main

import (
	"fmt"
	"log"
	"io"
	"github.com/gliderlabs/ssh"
)

func main(){
	ssh.Handle(func(s ssh.Session){
		if w,ok:=s.(io.Writer);ok{
			io.WriteString(w,fmt.Sprintf("Hello %s\n",s.User()))
		}
		log.Println("simple ssh server on port 2222")
		log.Fatal(ssh.ListenAndServe(":2222",nil))


	})

}


