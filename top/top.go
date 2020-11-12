package top

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/kent007/linux-inspect/pkg/fileutil"
)

// DefaultExecPath is the default 'top' command path.
var DefaultExecPath = "/usr/bin/top"

// Config configures 'top' command runs.
type Config struct {
	// Exec is the 'top' command path.
	// Defaults to '/usr/bin/top'.
	Exec string

	// MAKE THIS TRUE BY DEFAULT
	// OTHERWISE PARSER HAS TO DEAL WITH HIGHLIGHTED TEXTS
	//
	// BatchMode is true to start 'top' in batch mode, which could be useful
	// for sending output from 'top' to other programs or to a file.
	// In this mode, 'top' will not accept input and runs until the interations
	// limit ('-n' flag) or until killed.
	// It's '-b' flag.
	// BatchMode bool

	// Limit limits the iteration of 'top' commands to run before exit.
	// If 1, 'top' prints out the current processes and exits.
	// It's '-n' flag.
	Limit int

	// IntervalSecond is the delay time between updates.
	// Default is 1 second.
	// It's '-d' flag.
	IntervalSecond float64

	// PID specifies the PID to monitor.
	// It's '-p' flag.
	PID int64

	// Writer stores 'top' command outputs.
	Writer io.Writer

	cmd *exec.Cmd
}

// Flags returns the 'top' command flags.
func (cfg *Config) Flags() (fs []string) {
	// start 'top' in batch mode, which could be useful
	// for sending output from 'top' to other programs or to a file.
	// In this mode, 'top' will not accept input and runs until the interactions
	// limit ('-n' flag) or until killed.
	//
	// MAKE THIS TRUE BY DEFAULT
	// OTHERWISE PARSER HAS TO DEAL WITH HIGHLIGHTED TEXTS
	// add -w 512 to override command
	fs = append(fs, "-b", "-w", "512")

	if cfg.Limit > 0 { // if 1, command just exits after one output
		fs = append(fs, "-n", fmt.Sprintf("%d", cfg.Limit))
	}

	if cfg.IntervalSecond > 0 {
		fs = append(fs, "-d", fmt.Sprintf("%.2f", cfg.IntervalSecond))
	}

	if cfg.PID > 0 {
		fs = append(fs, "-p", fmt.Sprintf("%d", cfg.PID))
	}

	return
}

// process updates with '*exec.Cmd' for the given 'Config'.
func (cfg *Config) createCmd() error {
	if cfg == nil {
		return fmt.Errorf("Config is nil")
	}
	if !fileutil.Exist(cfg.Exec) {
		return fmt.Errorf("%q does not exist", cfg.Exec)
	}
	flags := cfg.Flags()

	c := exec.Command(cfg.Exec, flags...)
	c.Stdout = cfg.Writer
	c.Stderr = cfg.Writer

	cfg.cmd = c
	return nil
}

// Get returns all entries in 'top' command.
// If pid<1, it reads all processes in 'top' command.
// This is one-time command.
func Get(topPath string, pid int64, limit int, interval float64) ([]Row, int, error) {
	buf := new(bytes.Buffer)
	cfg := &Config{
		Exec:           topPath,
		Limit:          limit,
		IntervalSecond: interval,
		PID:            pid,
		Writer:         buf,
		cmd:            nil,
	}
	if cfg.Limit < 1 {
		cfg.Limit = 1
	}
	if cfg.IntervalSecond <= 0 {
		cfg.IntervalSecond = 1
	}

	if err := cfg.createCmd(); err != nil {
		return nil, -1, err
	}

	// run starts the 'top' command and waits for it to complete.
	if err := cfg.cmd.Run(); err != nil {
		return nil, -1, err
	}
	return Parse(buf.String())
}

// this version of TOP runs until a certain unix nano timestamp occurs, then terminates
// note that because of how top runs, the first measurement only take ~100ms and following measurements take
// 'interval' seconds
// this will theoretically run indefinitely -- not suggested for large timeouts, as the output buffer may become large
func GetTimed(topPath string, pid int64, stopTimestampNano int64, interval float64) ([]Row, int, error) {
	buf := new(bytes.Buffer)
	cfg := &Config{
		Exec:           topPath,
		Limit:          0,
		IntervalSecond: interval,
		PID:            pid,
		Writer:         buf,
		cmd:            nil,
	}
	if cfg.IntervalSecond <= 0 {
		cfg.IntervalSecond = 1
	}

	if err := cfg.createCmd(); err != nil {
		return nil, -1, err
	}
	duration := time.Until(time.Unix(0, stopTimestampNano))
	timeout := time.After(duration)

	result := make(chan error)

	_ = cfg.cmd.Start()
	//put the result from the command on a channel I can select against
	go func() {
		result <- cfg.cmd.Wait()
	}()

	select {
	case <-timeout:
		//this has a high possibility of happening anyway, we'll wait for exit to finish before we're done
		_ = cfg.cmd.Process.Kill()
		<-result
	case e := <-result:
		if e != nil {
			return nil, -1, e
		}
	}
	return Parse(buf.String())
}
