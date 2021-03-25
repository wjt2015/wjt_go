package main

/**
参考:
https://studygolang.com/articles/14926
https://studygolang.com/articles/11284
*/
import (
	"github.com/google/gopacket/pcap"
	"log"
)

func main() {

	pcapVersion := pcap.Version()

	log.Printf("pcapVersion=%+v\n", pcapVersion)

	ifs,err:=pcap.FindAllDevs()
	if err!=nil{

	}

	log.Printf("interfaces=%+v\n",ifs)

	handle,err:=pcap.OpenLive("eth0",100000,false,-1)

	defer handle.Close()

	handle.ReadPacketData()

}