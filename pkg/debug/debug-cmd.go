package debug

import (
	"fmt"
	"time"

	"github.com/buger/goterm"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/flant/shell-operator/pkg/app"
)

var (
	outputFormat  = "text"
	showEmpty     = false
	watch         = false
	watchInterval = "1s"
)

func DefineDebugCommands(kpApp *kingpin.Application) {
	// Queue dump commands.
	queueCmd := app.CommandWithDefaultUsageTemplate(kpApp, "queue", "Dump queues.")

	queueListCmd := queueCmd.Command("list", "Dump tasks in all queues.").
		Action(func(_ *kingpin.ParseContext) error {
			var refreshInterval time.Duration
			if watch {
				goterm.Clear()
				goterm.MoveCursor(1, 1)
				var err error
				refreshInterval, err = time.ParseDuration(watchInterval)
				if err != nil {
					goterm.Println(fmt.Sprintf("couldn't parse watch refresh interval: %s, default 1s applied\n", err))
					refreshInterval = time.Duration(1 * time.Second)
				}
			}
			for {
				out, err := Queue(DefaultClient()).List(outputFormat, showEmpty)
				if err != nil {
					return err
				}
				goterm.Println(string(out))
				goterm.Flush()

				if !watch {
					break
				}
				time.Sleep(refreshInterval)
				goterm.MoveCursor(1, 1)
				goterm.Clear()
			}
			return nil
		})
	queueListCmd.Flag("show-empty", "Show empty queues.").Short('e').
		Default("false").
		BoolVar(&showEmpty)
	queueListCmd.Flag("watch", "Keep watching.").Short('w').
		Default("false").
		BoolVar(&watch)
	queueListCmd.Flag("watchInterval", "Watch refresh interval.").Short('t').
		Default(watchInterval).
		StringVar(&watchInterval)
	AddOutputJsonYamlTextFlag(queueListCmd)
	app.DefineDebugUnixSocketFlag(queueListCmd)

	queueMainCmd := queueCmd.Command("main", "Dump tasks in the main queue.").
		Action(func(_ *kingpin.ParseContext) error {
			out, err := Queue(DefaultClient()).Main(outputFormat)
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		})
	AddOutputJsonYamlTextFlag(queueMainCmd)
	app.DefineDebugUnixSocketFlag(queueMainCmd)

	// Runtime config command.
	configCmd := app.CommandWithDefaultUsageTemplate(kpApp, "config", "Manage runtime parameters.")

	configListCmd := configCmd.Command("list", "List available runtime parameters.").
		Action(func(_ *kingpin.ParseContext) error {
			out, err := Config(DefaultClient()).List(outputFormat)
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		})
	AddOutputJsonYamlTextFlag(configListCmd)
	app.DefineDebugUnixSocketFlag(configListCmd)

	var paramName string
	var paramValue string
	var paramDuration time.Duration
	configSetCmd := configCmd.Command("set", "Set runtime parameter.").
		Action(func(_ *kingpin.ParseContext) error {
			out, err := Config(DefaultClient()).Set(paramName, paramValue, paramDuration)
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		})
	configSetCmd.Arg("name", "A name of runtime parameter").Required().StringVar(&paramName)
	configSetCmd.Arg("value", "A new value for the runtime parameter").Required().StringVar(&paramValue)
	configSetCmd.Arg("duration", "Set value for a period of time, then return a previous value. Use Go notation: 10s, 15m30s, etc.").DurationVar(&paramDuration)
	app.DefineDebugUnixSocketFlag(configSetCmd)

	// Raw request command
	var rawUrl string
	rawCommand := app.CommandWithDefaultUsageTemplate(kpApp, "raw", "Make a raw request to debug endpoint.").
		Action(func(_ *kingpin.ParseContext) error {
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

func DefineDebugCommandsSelf(kpApp *kingpin.Application) {
	// Get hook names
	hookCmd := app.CommandWithDefaultUsageTemplate(kpApp, "hook", "Actions for hooks")
	hookListCmd := hookCmd.Command("list", "List all hooks.").
		Action(func(_ *kingpin.ParseContext) error {
			outBytes, err := Hook(DefaultClient()).List(outputFormat)
			if err != nil {
				return err
			}
			fmt.Println(string(outBytes))
			return nil
		})
	AddOutputJsonYamlTextFlag(hookListCmd)
	app.DefineDebugUnixSocketFlag(hookListCmd)

	// Get hook snapshots
	var hookName string
	hookSnapshotCmd := hookCmd.Command("snapshot", "Dump hook snapshots.").
		Action(func(_ *kingpin.ParseContext) error {
			outBytes, err := Hook(DefaultClient()).Name(hookName).Snapshots(outputFormat)
			if err != nil {
				return err
			}
			fmt.Println(string(outBytes))
			return nil
		})
	hookSnapshotCmd.Arg("hook_name", "").Required().StringVar(&hookName)
	AddOutputJsonYamlTextFlag(hookSnapshotCmd)
	app.DefineDebugUnixSocketFlag(hookSnapshotCmd)
}

func AddOutputJsonYamlTextFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("output", "Output format: json|yaml|text.").Short('o').
		Default("text").
		EnumVar(&outputFormat, "json", "yaml", "text")
}

type QueueRequest struct {
	client *Client
}

func Queue(client *Client) *QueueRequest {
	return &QueueRequest{
		client: client,
	}
}

func (qr *QueueRequest) List(format string, showEmpty bool) ([]byte, error) {
	url := fmt.Sprintf("http://unix/queue/list.%s?showEmpty=%t", format, showEmpty)
	return qr.client.Get(url)
}

func (qr *QueueRequest) Main(format string) ([]byte, error) {
	url := fmt.Sprintf("http://unix/queue/main.%s", format)
	return qr.client.Get(url)
}

type HookRequest struct {
	client *Client
	name   string
}

func Hook(client *Client) *HookRequest {
	return &HookRequest{client: client}
}

func (r *HookRequest) Name(name string) *HookRequest {
	r.name = name
	return r
}

func (r *HookRequest) List(format string) ([]byte, error) {
	url := fmt.Sprintf("http://unix/hook/list.%s", format)
	return r.client.Get(url)
}

func (r *HookRequest) Snapshots(format string) ([]byte, error) {
	url := fmt.Sprintf("http://unix/hook/%s/snapshots.%s", r.name, format)
	return r.client.Get(url)
}

type ConfigRequest struct {
	client *Client
}

func Config(client *Client) *ConfigRequest {
	return &ConfigRequest{
		client: client,
	}
}

func (cr *ConfigRequest) List(format string) ([]byte, error) {
	url := fmt.Sprintf("http://unix/config/list.%s", format)
	return cr.client.Get(url)
}

func (cr *ConfigRequest) Set(name string, value string, duration time.Duration) ([]byte, error) {
	data := map[string][]string{
		"name":  {name},
		"value": {value},
	}
	if duration != 0 {
		data["duration"] = []string{duration.String()}
	}
	return cr.client.Post("http://unix/config/set", data)
}
