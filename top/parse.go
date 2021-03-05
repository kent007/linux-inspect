package top

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	humanize "github.com/dustin/go-humanize"
)

// parses memory bytes in top command,
// returns bytes in int64, and humanized bytes.
//
//  KiB = kibibyte = 1024 bytes
//  MiB = mebibyte = 1024 KiB = 1,048,576 bytes
//  GiB = gibibyte = 1024 MiB = 1,073,741,824 bytes
//  TiB = tebibyte = 1024 GiB = 1,099,511,627,776 bytes
//  PiB = pebibyte = 1024 TiB = 1,125,899,906,842,624 bytes
//  EiB = exbibyte = 1024 PiB = 1,152,921,504,606,846,976 bytes
//
func parseMemoryTxt(s string) (bts uint64, hs string, err error) {
	s = strings.TrimSpace(s)

	switch {
	case strings.HasSuffix(s, "m"): // suffix 'm' means megabytes
		ns := s[:len(s)-1]
		var mib float64
		mib, err = strconv.ParseFloat(ns, 64)
		if err != nil {
			return 0, "", err
		}
		bts = uint64(mib) * 1024 * 1024

	case strings.HasSuffix(s, "g"): // gigabytes
		ns := s[:len(s)-1]
		var gib float64
		gib, err = strconv.ParseFloat(ns, 64)
		if err != nil {
			return 0, "", err
		}
		bts = uint64(gib) * 1024 * 1024 * 1024

	case strings.HasSuffix(s, "t"): // terabytes
		ns := s[:len(s)-1]
		var tib float64
		tib, err = strconv.ParseFloat(ns, 64)
		if err != nil {
			return 0, "", err
		}
		bts = uint64(tib) * 1024 * 1024 * 1024 * 1024

	default:
		var kib float64
		kib, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, "", err
		}
		bts = uint64(kib) * 1024
	}

	hs = humanize.Bytes(bts)
	return
}

// Headers is the headers in 'top' output.
var Headers = []string{
	"PID",
	"USER",
	"PR",
	"NI",
	"VIRT",
	"RES",
	"SHR",
	"S",
	"%CPU",
	"%MEM",
	"TIME+",
	"COMMAND",
}

type commandOutputRowIdx int

const (
	command_output_row_idx_pid commandOutputRowIdx = iota
	command_output_row_idx_user
	command_output_row_idx_pr
	command_output_row_idx_ni
	command_output_row_idx_virt
	command_output_row_idx_res
	command_output_row_idx_shr
	command_output_row_idx_s
	command_output_row_idx_cpu
	command_output_row_idx_mem
	command_output_row_idx_time
	command_output_row_idx_command
)

//anything that might appear in the lines above the actual top output
var bytesToSkip = [][]byte{
	[]byte("top -"),
	[]byte("Tasks: "),
	[]byte("%Cpu(s): "),
	[]byte("Cpu(s): "),
	[]byte("KiB Mem :"),
	[]byte("KiB Swap :"),
	[]byte("Mem: "),
	[]byte("Swap: "),
	[]byte("MiB Mem :"),
	[]byte("MiB Swap:"),
	[]byte("PID "),
}

func topRowToSkip(data []byte) bool {
	for _, prefix := range bytesToSkip {
		if bytes.HasPrefix(data, prefix) {
			return true
		}
	}
	return false
}

// Parse parses 'top' command output and returns the rows.
func Parse(s string) ([]Row, int, error) {
	lines := strings.Split(s, "\n")
	rows := make([][]string, 0, len(lines))

	//start this one before, since the first measurement doesn't really count
	iterations := -1
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if topRowToSkip([]byte(line)) {
			//it's a header line, specifically keep track of each starting header for iteration count
			if strings.HasPrefix(line, "top -") {
				iterations++
			}
			continue
		}

		row := strings.Fields(strings.TrimSpace(line))
		if len(row) < len(Headers) {
			//too short
			return nil, iterations, fmt.Errorf("unexpected row column number %v (expected %v)", row, Headers)
		} else if len(row) > len(Headers) {
			//command had some spaces in it that got cut up into separate commands by the parser
			command := strings.Join(row[len(Headers)-1:], " ")
			row[len(row)-1] = command
		}
		//again, want to skip the first iteration, we don't care
		if iterations > 0 {
			rows = append(rows, row)
		}
	}

	type result struct {
		row Row
		err error
	}
	rc := make(chan result, len(rows))
	for _, row := range rows {
		go func(row []string) {
			tr, err := parseRow(row)
			rc <- result{row: tr, err: err}
		}(row)
	}

	tcRows := make([]Row, 0, len(rows))
	for len(tcRows) != len(rows) {
		select {
		case rs := <-rc:
			if rs.err != nil {
				return nil, iterations, rs.err
			}
			tcRows = append(tcRows, rs.row)
		}
	}
	if iterations == 0 {
		return tcRows, 0, fmt.Errorf("top did not take a significant measurement besides startup")
	}
	return tcRows, iterations, nil
}

func parseRow(row []string) (Row, error) {
	trow := Row{
		USER: strings.TrimSpace(row[command_output_row_idx_user]),
	}

	pv, err := strconv.ParseInt(row[command_output_row_idx_pid], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("parse error %v (row %v)", err, row)
	}
	trow.PID = pv

	trow.PR = strings.TrimSpace(row[command_output_row_idx_pr])
	trow.NI = strings.TrimSpace(row[command_output_row_idx_ni])

	virt, virtTxt, err := parseMemoryTxt(row[command_output_row_idx_virt])
	if err != nil {
		return Row{}, fmt.Errorf("parse error %v (row %v)", err, row)
	}
	trow.VIRT = row[command_output_row_idx_virt]
	trow.VIRTBytesN = virt
	trow.VIRTParsedBytes = virtTxt

	res, resTxt, err := parseMemoryTxt(row[command_output_row_idx_res])
	if err != nil {
		return Row{}, fmt.Errorf("parse error %v (row %v)", err, row)
	}
	trow.RES = row[command_output_row_idx_res]
	trow.RESBytesN = res
	trow.RESParsedBytes = resTxt

	shr, shrTxt, err := parseMemoryTxt(row[command_output_row_idx_shr])
	if err != nil {
		return Row{}, fmt.Errorf("parse error %v (row %v)", err, row)
	}
	trow.SHR = row[command_output_row_idx_shr]
	trow.SHRBytesN = shr
	trow.SHRParsedBytes = shrTxt

	trow.S = row[command_output_row_idx_s]
	trow.SParsedStatus = parseStatus(row[command_output_row_idx_s])

	cnum, err := strconv.ParseFloat(row[command_output_row_idx_cpu], 64)
	if err != nil {
		return Row{}, fmt.Errorf("parse error %v (row %v)", err, row)
	}
	trow.CPUPercent = cnum

	mnum, err := strconv.ParseFloat(row[command_output_row_idx_mem], 64)
	if err != nil {
		return Row{}, fmt.Errorf("parse error %v (row %v)", err, row)
	}
	trow.MEMPercent = mnum

	trow.TIME = row[command_output_row_idx_time]

	trow.COMMAND = row[command_output_row_idx_command]

	return trow, nil
}

func parseStatus(s string) string {
	ns := strings.TrimSpace(s)
	if len(s) > 1 {
		ns = ns[:1]
	}
	switch ns {
	case "D":
		return "D (uninterruptible sleep)"
	case "R":
		return "R (running)"
	case "S":
		return "S (sleeping)"
	case "T":
		return "T (stopped by job control signal)"
	case "t":
		return "t (stopped by debugger during trace)"
	case "Z":
		return "Z (zombie)"
	default:
		return fmt.Sprintf("unknown process %q", s)
	}
}
