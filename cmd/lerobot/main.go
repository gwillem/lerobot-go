package main

import (
	"os"

	"github.com/jessevdk/go-flags"
)

type Options struct {
	Setup       SetupCommand       `command:"setup" description:"Scan for arms and calibrate them"`
	Teleoperate TeleoperateCommand `command:"teleoperate" alias:"teleop" description:"Start teleoperation (leader-follower control)"`
}

var opts Options
var parser = flags.NewParser(&opts, flags.Default)

func main() {
	parser.LongDescription = "LeRobot - Robot arm control CLI for SO-101 arms"

	_, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok {
			if flagsErr.Type == flags.ErrHelp {
				os.Exit(0)
			}
		}
		os.Exit(1)
	}
}
