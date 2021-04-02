package main

import (
	"fmt"
	"github.com/gliderlabs/ssh"
	"io"
	"log"
)


// ListenAndServe listens on the TCP network address addr and then calls Serve
// with handler to handle sessions on incoming connections. Handler is typically
// nil, in which case the DefaultHandler is used.
func ListenAndServe(addr string, handler ssh.Handler, options ...ssh.Option) error {
	srv := &ssh.Server{Addr: addr, Handler: handler}
	for _, option := range options {
		if err := srv.SetOption(option); err != nil {
			return err
		}
	}
	log.Printf("srv=%+v\n",srv)
	return srv.ListenAndServe()
}

func main(){
	ssh.Handle(func(s ssh.Session){
		log.Printf("session=%+v\n",s)
		if w,ok:=s.(io.Writer);ok{
			log.Printf("w=%+v;ok=%+v;",w,ok)
			io.WriteString(w,fmt.Sprintf("Hello %s\n",s.User()))
		}
	})

	//ssh.HostKeyFile()

	log.Println("simple ssh server on port 2222")
	//log.Fatal(ssh.ListenAndServe(":2222",nil))
	log.Fatal(ListenAndServe(":2222",nil))


}


