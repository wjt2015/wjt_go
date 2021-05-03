package main

import "github.com/sirupsen/logrus"

func swap(arr [2]float64){
	logrus.Infof("inner!arr_addr=%p;arr=%+v\n",&arr,arr)
	arr[0]=10;
	arr[1]=15;
}

func incr(arr [] float64){
	logrus.Infof("incr();arr_addr=%p;arr=%+v;\n",&arr,arr)
	for i:=0;i<len(arr);i++ {
		arr[i]+=20;
	}
}

func main() {
	//pcap_base.PcapFunc()

	arr:=[2]float64{1,2}

	//arr:=make([]float64,4,6)

	logrus.Infof("before;arr=%+v;arr_addr=%p\n",arr,&arr)

	swap(arr)

	//incr(arr)

	logrus.Infof("after;arr=%+v\n",arr)
}



