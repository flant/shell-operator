package app

import "gopkg.in/alecthomas/kingpin.v2"

var JqLibraryPath = ""

// DefineJqFlags set flag for jq library
func DefineJqFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("jq-library-path", "Prepend directory to the search list for jq modules (-L flag). Can be set with $JQ_LIBRARY_PATH.").
		Envar("JQ_LIBRARY_PATH").
		Default(JqLibraryPath).
		StringVar(&JqLibraryPath)
}
