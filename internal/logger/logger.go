package logger

import (
	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger
var MainLog *logrus.Entry
var WebLog *logrus.Entry
var GitHubLog *logrus.Entry

func init() {
	Log = logrus.New()
	Log.SetReportCaller(false)

	MainLog = Log.WithFields(logrus.Fields{"category": "Main"})
	WebLog = Log.WithFields(logrus.Fields{"category": "WebServer"})
	GitHubLog = Log.WithFields(logrus.Fields{"category": "GitHub"})
}
