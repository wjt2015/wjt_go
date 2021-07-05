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
	elementArr, err := webdriver.FindElements(selenium.ByXPATH, "/screening/div.screening-bd/ul")

	logrus.Infof("elementArr=%+v",elementArr)

}


