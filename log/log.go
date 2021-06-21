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

var (
	L    logrus.Logger
	Path = "./logs"
)

func Init() {
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
		Level:        logrus.TraceLevel,
		ExitFunc: func(i int) {
			L.Warn(fmt.Sprintf("→→→→→→→→→→ this application is over by code %d ... ←←←←←←←←←←", i))
			os.Exit(i)
		},
	}

	filesMap := lfshook.WriterMap{}
	for k, v := range map[logrus.Level]string{
		logrus.InfoLevel:  "info",
		logrus.ErrorLevel: "error",
		logrus.FatalLevel: "fatal",
	} {

		f, err := rotatelogs.New(
			fmt.Sprintf("%s/%s-%s.log", Path, "%Y-%m-%d", v),
			rotatelogs.WithMaxAge(maxAge),
			rotatelogs.WithRotationTime(rotationTime),
		)
		if err != nil {
			log.Fatal(err)
		}
		filesMap[k] = f
	}

	filesMap[logrus.TraceLevel] = filesMap[logrus.InfoLevel]
	filesMap[logrus.DebugLevel] = filesMap[logrus.InfoLevel]

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
