package spider

import (
	"github.com/sirupsen/logrus"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
	"time"
)

func GetWebdriver() (selenium.WebDriver,error){
	cap:=selenium.Capabilities{
		"BrowserName":"chrome",
	}
	chromecaps:=chrome.Capabilities{
		Args:[]string{
			"--headless",
			"--no-sandbox",
			"--user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36", // 模拟user-agent，防反爬
		},
	}
	cap.AddChrome(chromecaps)
	return selenium.NewRemote(cap,"")
}
/**
微博超话;
 */
func WeiboChaoHua(webdriver selenium.WebDriver,url string){
	webdriver.SetPageLoadTimeout(2*time.Second)
	err:= webdriver.Get(url)
	source,err:=webdriver.PageSource()

	logrus.Info("source=%s;err=%+v",source,err)
}

func Weibo(){
	url:="https://m.weibo.cn/detail/4654928184216600"
	webdriver, err := GetWebdriver()
	logrus.Infof("webdriver=%+v;err=%+v;",webdriver,err)
	WeiboChaoHua(webdriver,url)
}



