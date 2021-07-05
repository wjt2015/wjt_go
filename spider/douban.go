package spider

import (
	"github.com/sirupsen/logrus"
	"github.com/tebeka/selenium"
	"time"
)

/**
豆瓣;
[https://movie.douban.com/]
 */
func Douban(){
	webdriver, err := GetWebdriver()
	logrus.Infof("webdriver=%+v;err=%+v;",webdriver,err)
	if err!=nil{
		return
	}
	defer webdriver.Quit()

	webdriver.SetPageLoadTimeout(2*time.Second)
	webdriver.Get("https://movie.douban.com/")

	pageSource, err := webdriver.PageSource()
	text:=string(pageSource)
	logrus.Infof("text=%s",text)

	elementArr, err := webdriver.FindElements(selenium.ByXPATH, "/screening/div.screening-bd/ul")

	logrus.Infof("elementArr=%+v",elementArr)

	element, err := webdriver.FindElement(selenium.ByXPATH, "//*[@id=\"screening\"]/div[2]/ul/li[46]/ul/li[1]/a/img")
	logrus.Infof("element=%+v;err=%+v;",element,err)

	element.Click()

	pageSource,err=webdriver.PageSource()
	text=string(pageSource)
	logrus.Infof("text=%s",text)

	element,err=webdriver.FindElement(selenium.ByXPATH,"//*[@id=\"content\"]")
	elementArr,err=element.FindElements(selenium.ByTagName,"span")
	logrus.Infof("elementArr=%+v",elementArr)

	for i,e:=range elementArr{
		text,err=e.Text()
		if text!=""{
			logrus.Infof("i=%d,text=%s;",i,text)

		}
	}

}


