
### 说明


https://www.selenium.dev/documentation/en/getting_started_with_webdriver/
https://github.com/tebeka/selenium

webdriver下载:  
http://npm.taobao.org/mirrors/chromedriver/


### golang selenium

golang Selenium WebDriver 使用记录
https://golangnote.com/topic/230.html

### golang selenium应用方法(以chrome browser为例)
根据本机chrome browser版本下载对应版本的webdriver,并安装,将路径加入PATH;
下载selenium-server-standalone-xxx.jar;
java -jar selenium-server-standalone-xxx.jar

获取webdriver实例的代码如下:
```
	capabilities  := selenium.Capabilities{"browserName": "chrome"}
	webDriver, err := selenium.NewRemote(capabilities, "")
	if err!=nil{
		logrus.Errorf("selenium.NewRemote error!err=%+v",err)
		return
	}
	logrus.Infof("webDriver=%+v;err=%+v;",webDriver,err)
	defer webDriver.Quit()
```
















