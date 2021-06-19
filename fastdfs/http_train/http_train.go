package main

import (
	"github.com/sirupsen/logrus"
)
/**
参考:
context.Context:
https://www.cnblogs.com/-lee/p/12820994.html
https://www.cnblogs.com/sunlong88/p/11272559.html
https://studygolang.com/articles/17082
https://zhuanlan.zhihu.com/p/110085652
 */
func init()  {
	logrus.SetReportCaller(true)
}


func main(){
	//httpServe()
	//chanFunc()
	//ctxFunc()
	//ctxFuncB()
	listen()
}




