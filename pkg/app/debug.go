package app

import (
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
)

var DebugUnixSocket = "/var/run/shell-operator/debug.socket"

var DebugKeepTmpFiles = "no"

var DebugKubernetesAPI = false

// SetupDebugSettings init global flags for debug
func DefineDebugFlags(kpApp *kingpin.Application, cmd *kingpin.CmdClause) {

	DefineDebugUnixSocketFlag(cmd)

	cmd.Flag("debug-keep-tmp-files", "set to yes to disable cleanup of temporary files").
		Envar("DEBUG_KEEP_TMP_FILES").
		Hidden().
		Default(DebugKeepTmpFiles).
		StringVar(&DebugKeepTmpFiles)

	cmd.Flag("debug-kubernetes-api", "enable client-go debug messages").
		Envar("DEBUG_KUBERNETES_API").
		Hidden().
		Default("false").
		BoolVar(&DebugKubernetesAPI)

	// A command to show help about hidden debug-* flags
	kpApp.Command("debug-options", "Show help for debug flags of a start command.").Hidden().PreAction(func(_ *kingpin.ParseContext) error {
		context, err := kpApp.ParseContext([]string{"start"})
		if err != nil {
			return err
		}

		// kingpin is used alecthomas/template instead of text/template,
		// so some wizardry is used to filter debug-* flags.
		usageTemplate := `{{define "FormatCommand"}}\
{{if .FlagSummary}} {{.FlagSummary}}{{end}}\
{{range .Args}} {{if not .Required}}[{{end}}<{{.Name}}>{{if .Value|IsCumulative}}...{{end}}{{if not .Required}}]{{end}}{{end}}\
{{end}}\

{{define "FormatUsage"}}\
{{template "FormatCommand" .}}{{if .Commands}} <command> [<args> ...]{{end}}
{{if .Help}}
{{.Help|Wrap 0}}\
{{end}}\
{{end}}\

usage: {{.App.Name}}{{template "FormatUsage" .App}}
Debug flags:
{{range .Context.Flags}}\
{{if and (ge .Name "debug-") (le .Name "debug-zzz") }}\
{{if .Short}}-{{.Short|Char}}, {{end}}--{{.Name}}{{if not .IsBoolFlag}}={{.FormatPlaceHolder}}{{end}}
        {{.Help}}
{{end}}\
{{end}}\
`

		if err := kpApp.UsageForContextWithTemplate(context, 2, usageTemplate); err != nil {
			panic(err)
		}

		os.Exit(0)
		return nil
	})
}

func DefineDebugUnixSocketFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("debug-unix-socket", "a path to a unix socket for a debug endpoint").
		Envar("DEBUG_UNIX_SOCKET").
		Hidden().
		Default(DebugUnixSocket).
		StringVar(&DebugUnixSocket)
}
