package validating_webhook

import (
	"crypto/x509"
	"fmt"
	"github.com/flant/shell-operator/pkg/app"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"path"
	"runtime"
	"testing"
)

func Test_ServerStart(t *testing.T) {
	app.ValidatingWebhookSettings.ServerKeyPath = "testdata/server-key.pem"
	app.ValidatingWebhookSettings.ServerCertPath = "testdata/server.crt"

	h := NewWebhookHandler()

	srv := &WebhookServer{Router: h.Router}

	err := srv.Start()
	if err != nil {
		t.Fatalf("Server should started: %v", err)
	}
}

func Test_Client_CA(t *testing.T) {
	roots := x509.NewCertPool()

	app.ValidatingWebhookSettings.ClientCAPaths = []string{
		"testdata/client-ca.pem",
	}

	for _, caPath := range app.ValidatingWebhookSettings.ClientCAPaths {
		caBytes, err := ioutil.ReadFile(caPath)
		if err != nil {
			t.Fatalf("ca '%s' should be read: %v", caPath, err)
		}

		ok := roots.AppendCertsFromPEM(caBytes)
		if !ok {
			t.Fatalf("ca '%s' should be parsed", caPath)
		}
	}

}

type MyFormatter struct {
	*logrus.TextFormatter
	CallerLevels map[logrus.Level]bool
}

func (f *MyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	if _, has := f.CallerLevels[entry.Level]; !has {
		entry.Caller = nil
	}
	return f.TextFormatter.Format(entry)
}

func Test_Logrus(t *testing.T) {
	logrus.SetReportCaller(true)
	logrus.SetFormatter(&MyFormatter{
		TextFormatter: &logrus.TextFormatter{
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				_, filename := path.Split(f.File)
				filename = fmt.Sprintf("%s:%d", filename, f.Line)
				return "", filename
			},
		},
		CallerLevels: map[logrus.Level]bool{
			logrus.ErrorLevel: true,
			logrus.DebugLevel: true,
		},
	})

	logrus.Info("info level message")
	logrus.Error("error level message")
	logrus.Debug("debug level message")
}
