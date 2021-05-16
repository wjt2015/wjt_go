package main

import (
	"errors"
	"fmt"
	"os"
	"github.com/sirupsen/logrus"
	"strings"
)

const MOD_CONTENT_FMT string = "module %s\ngo 1.16\n\n"
const VENDOR_SRC_PREFIX string = "./vendor/src/"

const GITHUB_PREFIX string = "github.com"
const GOOGLE_PREFIX string = "code.google.com"

func goModExists(dir string) bool {

	if _, err := os.Stat(dir + "/go.mod"); err != nil {
		logrus.Errorf("stat;dir=%s;err=%+v", dir, err)
		return false
	}
	return true
}

func genGoMod(dir string) {

	if goModExists(dir) {
		return
	}
	entryArr := strings.Split(dir, "/")
	modName := entryArr[len(entryArr)-1]
	modContent := fmt.Sprintf(MOD_CONTENT_FMT, modName)

	modFileName := dir + "/go.mod"
	if f, err := os.OpenFile(modFileName, os.O_CREATE|os.O_RDWR, 0777); err != nil {
		logrus.Errorf("open file error!modFileName=%s;err=%+v;", modFileName, err)
		//panic(err)
	} else {
		f.WriteString(modContent)
		logrus.Infof("write %s/go.mod finish!modContent=%s", dir, modContent)
	}
}

func Replacement(old string) (string, error) {
	if old == "" {
		return "", errors.New("old should not be empty!")
	}
	replaceStr := ""
	if strings.HasPrefix(old, GITHUB_PREFIX) || strings.HasPrefix(old, GOOGLE_PREFIX) {
		if strings.HasPrefix(old, "github.com/docker/docker") {
			replaceStr = strings.Replace(old, "github.com/docker/docker", "../docker_v120", 1)
			replaceStr = strings.Replace(replaceStr, "//", "/", 1)
		} else {
			replaceStr = VENDOR_SRC_PREFIX + old
		}
		return replaceStr, nil
	}
	return "", errors.New(old + ";no need to replace!")
}

func TidyMod(importFileName string, modFileName string) {
	if f, err := os.OpenFile(importFileName, os.O_RDONLY, 0); err != nil {
		logrus.Errorf("open file(%s) error!err=%+v;", importFileName, err)
	} else {
		stat, _ := os.Stat(importFileName)
		buf := make([]byte, stat.Size())
		f.Read(buf)
		f.Close()

		content := string(buf)
		lines := strings.Split(content, "\n")

		modFile, _ := os.OpenFile(modFileName, os.O_CREATE|os.O_RDWR, 0777)

		modFile.WriteString("module docker_v120\ngo 1.16\n\n\nreplace(\n")

		for i, line := range lines {
			line = strings.TrimSpace(line)
			replaceStr, _ := Replacement(line)
			if replaceStr != "" {
				newLine := line + " => " + replaceStr + "\n"
				modFile.WriteString(newLine)

				genGoMod(replaceStr)

			} else {
				logrus.Errorf("i=%d;line=%s;replaceStr=%s;", i, line, replaceStr)
			}

		}
		modFile.WriteString("\n)\n")
		modFile.Close()

	}
}

func main() {

	var (
		importFileName string = "/Users/jintao9/linux2014/wjt_projs/docker_v120/vendor/src/new_import.txt"
		modFileName    string = "/Users/jintao9/linux2014/wjt_projs/docker_v120/go.mod"
	)
	TidyMod(importFileName, modFileName)

}
