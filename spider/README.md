
### 说明


https://www.selenium.dev/documentation/en/getting_started_with_webdriver/
https://github.com/tebeka/selenium

webdriver下载:  
http://npm.taobao.org/mirrors/chromedriver/

---
### golang selenium

selenium go官网:  
https://github.com/tebeka/selenium  
https://www.w3.org/TR/webdriver/  
https://pkg.go.dev/github.com/tebeka/selenium   


golang Selenium WebDriver 使用记录  
https://golangnote.com/topic/230.html

Golang+selenium+chrome headless + goquery 在Linux 上作爬虫实践    
https://golangnote.com/topic/232.html
---

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


---

### centos下的golang+chromedriver

https://blog.csdn.net/wkb342814892/article/details/81591394
https://www.jianshu.com/p/32dd34da58cd

#### 安装chrome-browser
wget https://dl.google.com/linux/direct/google-chrome-stable_current_x86_64.rpm --no-check-certificate  
sudo yum install google-chrome-stable_current_x86_64.rpm
————————————————
版权声明：本文为CSDN博主「「已注销」」的原创文章，遵循CC 4.0 BY-SA版权协议，转载请附上原文出处链接及本声明。
原文链接：https://blog.csdn.net/wkb342814892/article/details/81591394
#### 下载并安装对应版本的chromedriver;

#### 示例代码  
```
//链接本地的浏览器 chrome
    caps := selenium.Capabilities{
        //"browserName": "/Applications/Google Chrome Dev.app/Contents/MacOS/Google Chrome Dev",
        "browserName": "Google Chrome Dev",
    }

    //禁止图片加载，加快渲染速度
    imagCaps := map[string]interface{}{
        "profile.managed_default_content_settings.images": 2,
    }
    chromeCaps := chrome.Capabilities{
        Prefs: imagCaps,
        Path:  "/Applications/Google Chrome Dev.app/Contents/MacOS/Google Chrome Dev",
        Args: []string{
            //静默执行请求
            "--headless", // 设置Chrome无头模式，在linux下运行，需要设置这个参数，否则会报错
            "--no-sandbox",
            "--user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36", // 模拟user-agent，防反爬
        },
    }
    //以上是设置浏览器参数
    caps.AddChrome(chromeCaps)


    url := "xxx"
    w_b1, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
```

代码示例2  
```
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
	
```


## 启动方法:
```
wjt_go目录下:
java -jar spider/selenium-server-standalone-3.13.0.jar

go run -mod=mod ./spider/cmd/
```
----












