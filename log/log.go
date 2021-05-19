package log

import (
	"fmt"
	"github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	maxAge       = 30 * time.Hour * 24
	rotationTime = time.Hour * 24
	timeFormat   = "2006-01-02 15:04:05"
)

var L logrus.Logger

//func I(arg ...interface{}) {
//	L.Info(arg)
//}
//
//func W(arg ...interface{}) {
//	L.Warn(arg)
//}
//
//func E(arg ...interface{}) {
//	L.Error(arg)
//}
//
//func F(arg ...interface{}) {
//	L.Fatal(arg)
//}
//
//func P(arg ...interface{}) {
//	L.Panic(arg)
//}

func init() {
	//控制台logger
	L = logrus.Logger{
		Out:   os.Stdout,
		Hooks: make(logrus.LevelHooks),
		Formatter: &logrus.TextFormatter{
			ForceColors:               true,
			ForceQuote:                true,
			EnvironmentOverrideColors: true,
			FullTimestamp:             true,
			TimestampFormat:           timeFormat,
			DisableSorting:            false,
			PadLevelText:              true,
			QuoteEmptyFields:          true,
			CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
				return frame.Function[strings.LastIndex(frame.Function, "/")+1:],
					fmt.Sprintf("%s:%d", filepath.Base(frame.File), frame.Line)
			},
		},
		ReportCaller: true,
		Level:        logrus.InfoLevel,
		ExitFunc: func(i int) {
			fmt.Println("the log exit!")
			os.Exit(i)
		},
	}

	filesMap := lfshook.WriterMap{}
	for k, v := range map[string]logrus.Level{
		"info":  logrus.InfoLevel,
		"error": logrus.ErrorLevel,
		"fatal": logrus.FatalLevel,
	} {
		f, err := rotatelogs.New(
			"./logs/%Y-%m-%d/"+fmt.Sprintf("%s.log", k),
			rotatelogs.WithMaxAge(maxAge),
			rotatelogs.WithRotationTime(rotationTime),
		)
		if err != nil {
			log.Fatal(err)
		}
		filesMap[v] = f
	}

	//文件日志
	L.AddHook(lfshook.NewHook(filesMap, &logrus.TextFormatter{
		FullTimestamp:    true,
		ForceQuote:       true,
		PadLevelText:     true,
		QuoteEmptyFields: true,
		TimestampFormat:  timeFormat,
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			return frame.Function[strings.LastIndex(frame.Function, "/")+1:],
				fmt.Sprintf("%s:%d", filepath.Base(frame.File), frame.Line)
		},
	}))
}
