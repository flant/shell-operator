package debug

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/flant/shell-operator/pkg/app"
)

var OutputFormat = "text"

func DefineDebugCommands(kpApp *kingpin.Application) {
	// Queue mamanging commands
	queueCmd := app.CommandWithDefaultUsageTemplate(kpApp, "queue", "Manage queues.")

	queueListCmd := queueCmd.Command("list", "Dump tasks in all queues.").
		Action(func(c *kingpin.ParseContext) error {
			queueDump, err := Queue(DefaultClient()).Dump(OutputFormat)
			if err != nil {
				return err
			}
			fmt.Println(string(queueDump))
			return nil
		})
	AddOutputJsonYamlTextFlag(queueListCmd)
	app.DefineDebugUnixSocketFlag(queueListCmd)

	// Raw request command
	var rawUrl string
	rawCommand := app.CommandWithDefaultUsageTemplate(kpApp, "raw", "Make a raw request to debug endpoint.").
		Action(func(c *kingpin.ParseContext) error {
			url := fmt.Sprintf("http://unix%s", rawUrl)
			resp, err := DefaultClient().Get(url)
			if err != nil {
				return err
			}
			fmt.Println(string(resp))
			return nil
		})
	rawCommand.Arg("urlpath", "An url to send to debug endpoint. Example: /queue/list.json").StringVar(&rawUrl)
	app.DefineDebugUnixSocketFlag(rawCommand)
}

func AddOutputJsonYamlTextFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("output", "Output format: json|yaml|text.").Short('o').
		Default("text").
		EnumVar(&OutputFormat, "json", "yaml", "text")
}

type QueueRequest struct {
	client *Client
}

func Queue(client *Client) *QueueRequest {
	return &QueueRequest{
		client: client,
	}
}

func (qr *QueueRequest) Dump(format string) ([]byte, error) {
	url := fmt.Sprintf("http://unix/queue/list.%s", format)
	return qr.client.Get(url)
}
