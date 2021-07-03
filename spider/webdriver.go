package spider

import (
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
)

func getWebdriver() (selenium.WebDriver,error){
	cap:=selenium.Capabilities{
		"BrowserName":"chrome",
	}
	chromecaps:=chrome.Capabilities{
		Args:[]string{
			"--headerless",
			"",
		},
	}
}


