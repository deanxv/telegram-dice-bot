package main

import (
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"telegram-dice-bot/internal/bot"
)

func main() {
	setupLogging()

	bot.StartBot()

}
func setupLogging() {
	// 设置日志级别
	logrus.SetLevel(logrus.InfoLevel) // 或根据环境变量设置

	// 设置日志格式为
	formatter := &logrus.TextFormatter{
		// 开启彩色输出
		ForceColors:   true,
		FullTimestamp: true,
	}

	// 设置日志格式为自定义的TextFormatter
	logrus.SetFormatter(formatter)

	// 创建一个日志文件输出
	fileOutput := &lumberjack.Logger{
		Filename:   "logs/myapp.log",
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   //days
		Compress:   true, // 压缩旧日志文件
	}
	logrus.SetOutput(io.MultiWriter(fileOutput, os.Stdout))

	// 记录日志发生的文件名和行号
	logrus.SetReportCaller(true)
}
