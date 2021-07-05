package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
	"net"
	"os"
	"time"
	"wjt_go/spider"
)

func pickUnusedPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		return 0, err
	}
	return port, nil
}

func init(){
	logrus.SetReportCaller(true)
}

func main(){
	//webdriverA()
	//spider.Weibo()
	spider.Douban()

}

func webdriverA(){

	capabilities  := selenium.Capabilities{"browserName": "chrome"}
	chromecaps:=chrome.Capabilities{
		Args: []string{
			//静默执行请求
			"--headless", // 设置Chrome无头模式，在linux下运行，需要设置这个参数，否则会报错
			"--no-sandbox",
			"--user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36", // 模拟user-agent，防反爬
		},
	}
	capabilities.AddChrome(chromecaps)

	webDriver, err := selenium.NewRemote(capabilities, "")
	if err!=nil{
		logrus.Errorf("selenium.NewRemote error!err=%+v",err)
		return
	}
	logrus.Infof("webDriver=%+v;err=%+v;",webDriver,err)
	defer webDriver.Quit()
	webDriver.Get("https://baijiahao.baidu.com/s?id=1628782259102304673&wfr=spider&for=pc")

	time.Sleep(100*time.Second)
}

/**
https://golangnote.com/topic/230.html
 */
func test2() {
	port, err := pickUnusedPort()
	logrus.Infof("port=", port)

	opts := []selenium.ServiceOption{
		selenium.Output(os.Stderr),
	}
	selenium.SetDebug(false)


	//service, err := selenium.NewIeDriverService("IEDriverServer.exe", port, opts...)
	service, err := selenium.NewChromeDriverService("IEDriverServer.exe", port, opts...)
	if err != nil {
		panic(err)
	}
	defer service.Stop()

	fmt.Println("here 1")

	// 起新线程在新标签页打开窗口
	go func() {
		caps := selenium.Capabilities{"browserName": "internet explorer"}
		wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d", port))
		if err != nil {
			panic(err)
		}
		defer wd.Quit()

		if err := wd.Get("https://www.baidu.com"); err != nil {
			panic(err)
		}

		time.Sleep(time.Second * 10)

		// 在窗口调用js 脚本
		wd.ExecuteScript(`window.open("https://www.qq.com", "_blank");`, nil)

		time.Sleep(time.Second * 20)
	}()

	// 打开一个在一个标签页里打开一个窗口
	caps := selenium.Capabilities{"browserName": "internet explorer"}
	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d", port))
	if err != nil {
		panic(err)
	}
	defer wd.Quit()

	// Navigate to the simple playground interface.
	if err := wd.Get("http://play.golang.org/?simple=1"); err != nil {
		panic(err)
	}

	// Get a reference to the text box containing code.
	elem, err := wd.FindElement(selenium.ByCSSSelector, "#code")
	if err != nil {
		panic(err)
	}
	// Remove the boilerplate code already in the text box.
	if err := elem.Clear(); err != nil {
		panic(err)
	}

	// Enter some new code in text box.
	err = elem.SendKeys(`
		package main
		import "fmt"
		func main() {
			fmt.Println("Hello WebDriver!\n")
		}
	`)
	if err != nil {
		panic(err)
	}

	// Click the run button.
	btn, err := wd.FindElement(selenium.ByCSSSelector, "#run")
	if err != nil {
		panic(err)
	}
	if err := btn.Click(); err != nil {
		panic(err)
	}

	// Wait for the program to finish running and get the output.
	outputDiv, err := wd.FindElement(selenium.ByCSSSelector, "#output")
	if err != nil {
		panic(err)
	}

	var output string
	for {
		output, err = outputDiv.Text()
		if err != nil {
			panic(err)
		}
		if output != "Waiting for remote server..." {
			break
		}
		time.Sleep(time.Second * 1)
	}

	fmt.Println("waiting....")
	time.Sleep(time.Second * 80)

	// Example Output:
	// Hello WebDriver!
	//
	// Program exited.
}


