package app

import "gopkg.in/alecthomas/kingpin.v2"

func SetupStartCommandFlags(kpApp *kingpin.Application, cmd *kingpin.CmdClause) {
	DefineAppFlags(cmd)
	DefineJqFlags(cmd)
	DefineLoggingFlags(cmd)
	DefineDebugFlags(kpApp, cmd)
}
