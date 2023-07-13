//go:generate go run templates/templates_gen.go -t templates -o pkg/templates.go
package main

import (
	"separa/common"
	"separa/common/flag"
	"separa/common/log"
	"separa/core/plugin"
	"separa/core/run"
	"time"
)

func ConfigInit() {
	common.Setting = common.New()
	common.Setting.Target = flag.Targets
	common.Setting.Output = flag.Command.OutputFile
	common.Setting.LoadPort(flag.Command.Port)

	plugin.RunOpt.Delay = flag.Command.Delay
	plugin.RunOpt.HttpsDelay = flag.Command.Delay / 2
	plugin.RunOpt.Debug = flag.Command.Debug
}

func main() {
	startTime := time.Now()
	// First we parse the args
	flag.Parse()

	// Use flag to set config
	ConfigInit()

	// Then we start the main process
	run.Start(&flag.Targets)

	// Finally we print the elapsed time
	elapsed := time.Since(startTime)
	log.Log.Printf("All tasks down, costs %s", elapsed)
}
